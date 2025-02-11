package main

import (
	"io"
	"math/rand"
	"time"

	"github.com/Joe-TheBro/scalingfake/shared/config"
	"github.com/Joe-TheBro/scalingfake/shared/utils"
	"github.com/charmbracelet/log"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"gocv.io/x/gocv"
)

func startWebrtcClient(signalingctxSSH *utils.SSHContext) {
	sshClient := signalingctxSSH.SSHClient
	session, err := sshClient.NewSession()
	if err != nil {
		log.Fatalf("Error creating session: %v", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		log.Fatalf("Error getting stdin pipe: %v", err)
	}
	defer stdin.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		log.Fatalf("Error getting stdout pipe: %v", err)
	}

	if err := session.Start("webrtc-signal"); err != nil {
		log.Fatalf("Error starting webrtc-signal: %v", err)
	}

	pc, err := CreatePeerConnection()
	if err != nil {
		log.Fatalf("Error creating peer connection: %v", err)
	}
	defer pc.Close()

	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		log.Info("Recieved remote track from server")
		go displayRemoteTrack(track)
	})

	localTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "pion")
	if err != nil {
		log.Fatalf("Error creating local track: %v", err)
	}
	if _, err = pc.AddTrack(localTrack); err != nil {
		log.Fatalf("Error adding local track: %v", err)
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		log.Fatalf("Error creating offer: %v", err)
	}
	if err = pc.SetLocalDescription(offer); err != nil {
		log.Fatalf("Error setting local description: %v", err)
	}

	<-webrtc.GatheringCompletePromise(pc)

	sdpOffer := pc.LocalDescription().SDP
	log.Info("Sending offer to server")
	log.Infof("Offer: %s", sdpOffer)
	if _, err = stdin.Write([]byte(sdpOffer)); err != nil {
		log.Fatalf("Error writing offer to stdin: %v", err)
	}

	answerBuf := make([]byte, 4096)
	n, err := stdout.Read(answerBuf)
	if err != nil && err != io.EOF {
		log.Fatalf("Error reading answer: %v", err)
	}
	sdpAnswer := string(answerBuf[:n])
	log.Info("Received SDP answer from signaling server")
	log.Infof("Answer: %s", sdpAnswer)

	answerDesc := webrtc.SessionDescription{
		Type: webrtc.SDPTypeAnswer,
		SDP:  sdpAnswer,
	}
	if err = pc.SetRemoteDescription(answerDesc); err != nil {
		log.Fatalf("Error setting remote description: %v", err)
	}
	
	go captureAndSendLocalVideo(localTrack)

	select {}
}

func CreatePeerConnection() (*webrtc.PeerConnection, error) {
	var m webrtc.MediaEngine
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:     "video/H264",
			ClockRate:    90000,
		},
	}, webrtc.RTPCodecTypeVideo); err != nil {
		log.Error("Error registering codec: %v", err)
		return nil, err
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(&m))
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
	return api.NewPeerConnection(config)
}

func captureAndSendLocalVideo(track *webrtc.TrackLocalStaticRTP) {
	// Open webcam
	capture, err := gocv.OpenVideoCapture(config.CameraIndex)
	if err != nil {
		log.Fatalf("Error opening video capture: %v", err)
	}
	defer capture.Close()

	window := gocv.NewWindow("Local Video (Sending)")
	defer window.Close()

	img := gocv.NewMat()
	defer img.Close()

	var sequenceNumber uint16 = 0
	var timestamp uint32 = 0
	ssrc := uint32(rand.Uint32())

	fps := 30
	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	for range ticker.C {
		if ok := capture.Read(&img); !ok {
			log.Warn("Error reading frame from webcam")
			continue
		}
		if img.Empty() {
			continue
		}

		window.IMShow(img)
		window.WaitKey(1)

		buf, err := gocv.IMEncode(".jpg", img)
		if err != nil {
			log.Fatalf("Error encoding image: %v", err)
		}
		
		jpegBytes := buf.GetBytes()
		buf.Close()

		packet := &rtp.Packet{
			Header: rtp.Header{
				Version:        2,
				Marker:         true,
				PayloadType:   96, // dynamic payload type
				SequenceNumber: sequenceNumber,
				Timestamp:      timestamp,
				SSRC:           ssrc,
			},
			Payload: jpegBytes,
		}
		sequenceNumber++
		timestamp += 90000 / uint32(fps)

		if err = track.WriteRTP(packet); err != nil {
			log.Errorf("Error writing RTP packet: %v", err)
		}
	}
}

func displayRemoteTrack(track *webrtc.TrackRemote) {
	window := gocv.NewWindow("Remote Video (Receiving)")
	defer window.Close()

	for {
		packet, _, err := track.ReadRTP()
		if err != nil {
			log.Error("Error reading RTP packet: %v", err)
			continue
		}

		img, err := gocv.IMDecode(packet.Payload, gocv.IMReadColor)
		if err != nil {
			log.Error("Error decoding image: %v", err)
			continue
		}

		if img.Empty() {
			continue
		}

		window.IMShow(img)
		if window.WaitKey(1) >= 0 {
			break
		}
		img.Close()
	}
}