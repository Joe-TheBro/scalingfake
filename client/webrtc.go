package main

import (
	"io"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/Joe-TheBro/scalingfake/shared/config"
	"github.com/Joe-TheBro/scalingfake/shared/utils"
	"github.com/charmbracelet/log"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"gocv.io/x/gocv"
)

type bufferedPacket struct {
	packet  *rtp.Packet
	arrival time.Time
}

// JitterBuffer buffers RTP packets for a fixed delay to allow reordering.
type JitterBuffer struct {
	inputChan  chan *rtp.Packet
	outputChan chan *rtp.Packet
	maxDelay   time.Duration

	mu     sync.Mutex
	buffer []bufferedPacket
}

var (
	latestLocalFrame gocv.Mat = gocv.NewMat()
	latestLocalFrameMu sync.RWMutex
	latestRemoteFrame gocv.Mat = gocv.NewMat()
	latestRemoteFrameMu sync.RWMutex
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

	localTrack, err := webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{MimeType: "video/jpeg"}, "video", "pion")
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
			MimeType:     "video/jpeg",
			ClockRate:    90000,
		},
		PayloadType: 97,
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

// captureAndSendLocalVideo encodes raw webcam frames to VP8 using ffmpeg (outputting IVF)
// and then packetizes each VP8 frame into RTP packets.
func captureAndSendLocalVideo(track *webrtc.TrackLocalStaticRTP) {
	// Open the webcam.
	capture, err := gocv.OpenVideoCapture(config.CameraIndex)
	if err != nil {
		log.Fatalf("Error opening video capture: %v", err)
	}
	defer capture.Close()

	capture.Set(gocv.VideoCaptureFrameWidth, 1280)
	capture.Set(gocv.VideoCaptureFrameHeight, 720)

	// Set the target frame rate
	fps := 60
	maxPayloadSize := 1200 // Maximum RTP payload size in bytes.

	// send frame to local window display
	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(fps))
		defer ticker.Stop()

		for range ticker.C {
			img := gocv.NewMat()
			if ok := capture.Read(&img); !ok || img.Empty() {
				log.Error("Error reading frame from capture")
				img.Close()
				continue
			}

			latestLocalFrameMu.Lock()
			if !latestLocalFrame.Empty() {
				latestLocalFrame.Close()
			}
			latestLocalFrame = img.Clone()
			latestLocalFrameMu.Unlock()

			img.Close()
		}
	}()

	// RTP state variables.
	var sequenceNumber uint16 = 0
	var timestamp uint32 = 0
	ssrc := uint32(rand.Uint32())

	go func ()  {
		ticker := time.NewTicker(time.Second / time.Duration(fps))
		defer ticker.Stop()
	
		for range ticker.C {
			// grab latest frame
			latestLocalFrameMu.RLock()
			img := latestLocalFrame.Clone()
			latestLocalFrameMu.RUnlock()
	
			// check if the frame is empty
			if img.Empty() {
				img.Close()
				continue
			}
	
			jpegBuf, err := gocv.IMEncode(".jpg", img)
			if err != nil {
				log.Errorf("Error encoding image: %v", err)
				img.Close()
				continue
			}
			img.Close()
	
			jpegBytes := make([]byte, jpegBuf.Len())
			copy(jpegBytes, jpegBuf.GetBytes())
			jpegBuf.Close()
	
			packets := packetizeJPEG(jpegBytes, maxPayloadSize)
			for i, payload := range packets {
				marker := false
				// set the marker bit on the last packet of the frame
				if i == len(packets)-1 {
					marker = true
				}
	
				rtpPacket := &rtp.Packet{
					Header: rtp.Header{
						Version:        2,
						PayloadType:	97, // Use a dynamic payload type (96-127) for JPEG
						SequenceNumber: sequenceNumber,
						Timestamp:      timestamp,
						SSRC:           ssrc,
						Marker:         marker,
					},
					Payload: payload,
				}
				sequenceNumber++
			
				if err := track.WriteRTP(rtpPacket); err != nil {
					log.Errorf("Error writing RTP packet: %v", err)
				}
			}
	
		timestamp += 90000 / uint32(fps)
		}
	}()
	
	// block forever
	select {}
}

func packetizeJPEG(jpegData []byte, maxPayloadSize int) [][]byte {
    const jpegHeaderSize = 8
    var packets [][]byte
    dataLen := len(jpegData)
    offset := 0

    // Determine the header values. These may come from the JPEG's metadata or be set statically.
    typeSpecific := byte(0)
    jpegType := byte(1)     // Example value; choose the one that matches your use case.
    quality := byte(255)    // Maximum quality by default.
    width := byte(1280 / 8) // In 8-pixel units.
    height := byte(720 / 8)

    for offset < dataLen {
        // Reserve space for header.
        chunkSize := maxPayloadSize - jpegHeaderSize
        if offset+chunkSize > dataLen {
            chunkSize = dataLen - offset
        }

        // Create header.
        header := make([]byte, jpegHeaderSize)
        header[0] = typeSpecific
        // Fragment offset is 3 bytes; big endian.
        header[1] = byte((offset >> 16) & 0xFF)
        header[2] = byte((offset >> 8) & 0xFF)
        header[3] = byte(offset & 0xFF)
        header[4] = jpegType
        header[5] = quality
        header[6] = width
        header[7] = height

        // Combine header with the JPEG chunk.
        packet := make([]byte, jpegHeaderSize+chunkSize)
        copy(packet[:jpegHeaderSize], header)
        copy(packet[jpegHeaderSize:], jpegData[offset:offset+chunkSize])
        packets = append(packets, packet)

        offset += chunkSize
    }
    return packets
}

func isValidJPEG(data []byte) bool {
    if len(data) < 4 {
        return false
    }
    return data[0] == 0xFF && data[1] == 0xD8 &&
           data[len(data)-2] == 0xFF && data[len(data)-1] == 0xD9
}

// NewJitterBuffer creates a new jitter buffer with the given maximum delay.
func NewJitterBuffer(maxDelay time.Duration) *JitterBuffer {
	jb := &JitterBuffer{
		inputChan:  make(chan *rtp.Packet, 100),
		outputChan: make(chan *rtp.Packet, 100),
		maxDelay:   maxDelay,
		buffer:     []bufferedPacket{},
	}
	go jb.run()
	return jb
}

func (jb *JitterBuffer) run() {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case pkt := <-jb.inputChan:
			jb.mu.Lock()
			jb.buffer = append(jb.buffer, bufferedPacket{packet: pkt, arrival: time.Now()})
			jb.mu.Unlock()
		case <-ticker.C:
			now := time.Now()
			var ready []bufferedPacket
			jb.mu.Lock()
			var remaining []bufferedPacket
			for _, bp := range jb.buffer {
				if now.Sub(bp.arrival) >= jb.maxDelay {
					ready = append(ready, bp)
				} else {
					remaining = append(remaining, bp)
				}
			}
			jb.buffer = remaining
			jb.mu.Unlock()

			if len(ready) > 0 {
				// Sort the ready packets by sequence number.
				sort.Slice(ready, func(i, j int) bool {
					return ready[i].packet.SequenceNumber < ready[j].packet.SequenceNumber
				})
				for _, bp := range ready {
					jb.outputChan <- bp.packet
				}
			}
		}
	}
}

// Input returns the input channel to feed RTP packets into.
func (jb *JitterBuffer) Input() chan<- *rtp.Packet {
	return jb.inputChan
}

// Output returns the output channel from which sorted RTP packets can be read.
func (jb *JitterBuffer) Output() <-chan *rtp.Packet {
	return jb.outputChan
}

// func displayRemoteTrack(track *webrtc.TrackRemote) {
// 	fragmentBuffer := make(map[int][]byte)
// 	expectedTotalSize := -1
// 	complete := false
// 	var frameData []byte

// 	for {
// 		packet, _ , err := track.ReadRTP()
// 		if err != nil {
// 			log.Error("Error reading RTP packet: %v", err)
// 			continue
// 		}

// 		if len(packet.Payload) < 8 {
// 			log.Error("packet too small to extract JPEG header")
// 			continue
// 		}

// 		header := packet.Payload[:8]
// 		fragmentOffset := int(header[1])<<16 | int(header[2])<<8 | int(header[3])
// 		payload := packet.Payload[8:]

// 		fragmentBuffer[fragmentOffset] = payload

// 		if packet.Marker {
// 			expectedTotalSize = fragmentOffset + len(payload)
// 		}

// 		// if we have all the fragments, reconstruct the frame
// 		if expectedTotalSize > 0 {
// 			frameData = make([]byte, expectedTotalSize)
// 			complete = true

// 			for offset := 0; offset < expectedTotalSize; {
// 				frag, exists := fragmentBuffer[offset]
// 				if !exists {
// 					complete = false
// 					break
// 				}
// 				copy(frameData[offset:], frag)
// 				offset += len(frag)
// 			}
// 		}

// 		if complete {
// 			if !isValidJPEG(frameData) {
// 				log.Error("Invalid JPEG frame")
// 			} else {
// 				img, err := gocv.IMDecode(frameData, gocv.IMReadColor)
// 				if err != nil {
// 					log.Error("Error decoding image: %v", err)
// 				}

// 				if img.Empty() {
// 					log.Debug("(REMOTE) Empty image")
// 				} else {
// 					latestRemoteFrameMu.Lock()
// 					oldFrame := latestRemoteFrame
// 					latestRemoteFrame = img.Clone()
// 					latestRemoteFrameMu.Unlock()
// 					if !oldFrame.Empty() {
// 						oldFrame.Close()
// 					}

// 					img.Close()
// 				}
// 			}
// 			fragmentBuffer = make(map[int][]byte)
// 			expectedTotalSize = -1
// 		}
// 	}
// }

// func displayRemoteTrack(track *webrtc.TrackRemote) {
// 	// Create a jitter buffer with a 50ms delay (adjust as needed).
// 	jb := NewJitterBuffer(100 * time.Millisecond)

// 	// Read packets from the track and feed them into the jitter buffer.
// 	go func() {
// 		for {
// 			packet, _, err := track.ReadRTP()
// 			if err != nil {
// 				log.Errorf("Error reading RTP packet: %v", err)
// 				continue
// 			}
// 			jb.Input() <- packet
// 		}
// 	}()

// 	// Variables for JPEG reassembly.
// 	fragmentBuffer := make(map[int][]byte)
// 	expectedTotalSize := -1
// 	var frameData []byte

// 	// Process packets from the jitter buffer.
// 	for packet := range jb.Output() {
// 		if len(packet.Payload) < 8 {
// 			log.Error("packet too small to extract JPEG header")
// 			continue
// 		}

// 		header := packet.Payload[:8]
// 		fragmentOffset := int(header[1])<<16 | int(header[2])<<8 | int(header[3])
// 		payload := packet.Payload[8:]

// 		fragmentBuffer[fragmentOffset] = payload

// 		if packet.Marker {
// 			expectedTotalSize = fragmentOffset + len(payload)
// 		}

// 		// Try reassembling when we know the total size.
// 		if expectedTotalSize > 0 {
// 			frameData = make([]byte, expectedTotalSize)
// 			complete := true
// 			for offset := 0; offset < expectedTotalSize; {
// 				frag, exists := fragmentBuffer[offset]
// 				if !exists {
// 					complete = false
// 					break
// 				}
// 				copy(frameData[offset:], frag)
// 				offset += len(frag)
// 			}

// 			if complete {
// 				if !isValidJPEG(frameData) {
// 					log.Error("Invalid JPEG frame")
// 				} else {
// 					img, err := gocv.IMDecode(frameData, gocv.IMReadColor)
// 					if err != nil {
// 						log.Error("Error decoding image: %v", err)
// 					} else if img.Empty() {
// 						log.Debug("(REMOTE) Empty image")
// 					} else {
// 						latestRemoteFrameMu.Lock()
// 						oldFrame := latestRemoteFrame
// 						latestRemoteFrame = img.Clone()
// 						latestRemoteFrameMu.Unlock()
// 						if !oldFrame.Empty() {
// 							oldFrame.Close()
// 						}
// 					}
// 				}
// 				// Reset for the next frame.
// 				fragmentBuffer = make(map[int][]byte)
// 				expectedTotalSize = -1
// 			}
// 		}
// 	}
// }

func displayRemoteTrack(track *webrtc.TrackRemote) {
	// Create a jitter buffer with a 100ms delay (adjust as needed).
	jb := NewJitterBuffer(100 * time.Millisecond)

	// Read packets from the track and feed them into the jitter buffer.
	go func() {
		for {
			packet, _, err := track.ReadRTP()
			if err != nil {
				log.Errorf("Error reading RTP packet: %v", err)
				continue
			}
			jb.Input() <- packet
		}
	}()

	// Variables for JPEG reassembly.
	fragmentBuffer := make(map[int][]byte)
	expectedTotalSize := -1
	var frameData []byte
	var lastPacketTime time.Time

	// Process packets from the jitter buffer.
	for packet := range jb.Output() {
		if len(packet.Payload) < 8 {
			log.Error("packet too small to extract JPEG header")
			continue
		}

		header := packet.Payload[:8]
		fragmentOffset := int(header[1])<<16 | int(header[2])<<8 | int(header[3])
		payload := packet.Payload[8:]

		// Flush previous frame if a new one starts (fragmentOffset == 0) and buffer is not empty.
		if fragmentOffset == 0 && len(fragmentBuffer) > 0 {
			log.Warn("New frame detected. Flushing incomplete previous frame.")
			fragmentBuffer = make(map[int][]byte)
			expectedTotalSize = -1
		}

		// Store the fragment.
		fragmentBuffer[fragmentOffset] = payload

		// If marker is set, update the expected total size.
		if packet.Marker {
			expectedTotalSize = fragmentOffset + len(payload)
		}

		// Update last packet time and apply timeout recovery.
		now := time.Now()
		if lastPacketTime.IsZero() {
			lastPacketTime = now
		} else if now.Sub(lastPacketTime) > 100*time.Millisecond {
			// If frame is incomplete after timeout, flush the buffer.
			if expectedTotalSize > 0 {
				complete := true
				for offset := 0; offset < expectedTotalSize; {
					if frag, exists := fragmentBuffer[offset]; !exists {
						complete = false
						break
					} else {
						offset += len(frag)
					}
				}
				if !complete {
					log.Warn("Frame incomplete after timeout. Flushing buffer.")
					fragmentBuffer = make(map[int][]byte)
					expectedTotalSize = -1
				}
			}
			lastPacketTime = now
		} else {
			lastPacketTime = now
		}

		// Attempt to reassemble if we know the total size.
		if expectedTotalSize > 0 {
			complete := true
			frameData = make([]byte, expectedTotalSize)
			offset := 0
			for offset < expectedTotalSize {
				frag, exists := fragmentBuffer[offset]
				if !exists {
					complete = false
					break
				}
				copy(frameData[offset:], frag)
				offset += len(frag)
			}

			if complete {
				if !isValidJPEG(frameData) {
					log.Error("Invalid JPEG frame")
				} else {
					img, err := gocv.IMDecode(frameData, gocv.IMReadColor)
					if err != nil {
						log.Error("Error decoding image: %v", err)
					} else if img.Empty() {
						log.Debug("(REMOTE) Empty image")
					} else {
						latestRemoteFrameMu.Lock()
						oldFrame := latestRemoteFrame
						latestRemoteFrame = img.Clone()
						latestRemoteFrameMu.Unlock()
						img.Close()
						if !oldFrame.Empty() {
							oldFrame.Close()
						}
					}
				}
				// Reset for the next frame.
				fragmentBuffer = make(map[int][]byte)
				expectedTotalSize = -1
				lastPacketTime = time.Time{}
			}
		}
	}
}