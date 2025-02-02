package main

import (
	"encoding/binary"
	"encoding/json"
	"net"
	"os"

	"github.com/Joe-TheBro/scalingfake/shared/config"
	"github.com/Joe-TheBro/scalingfake/shared/security"
	"github.com/Joe-TheBro/scalingfake/shared/webrtc"
	"github.com/charmbracelet/log"
	pionWebRTC "github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"gocv.io/x/gocv"
)

func serverLocalDescription(conn net.Conn, encryptionKey []byte, peerConnection *pionWebRTC.PeerConnection) {
	// Create an offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		log.Fatal("Error creating offer:", err)
	}

	// Set the local description
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		log.Fatal("Error setting local description:", err)
	}

	// Wait for ICE gathering to complete
	gatherComplete := pionWebRTC.GatheringCompletePromise(peerConnection)
	<-gatherComplete

	// Get the local description
	localDesc := peerConnection.LocalDescription()

	// Convert the local description to JSON
	localDescJSON, err := json.Marshal(localDesc)
	if err != nil {
		log.Fatal("Error marshalling local description to JSON:", err)
	}

	// Encrypt the local description
	encryptedLocalDesc, err := security.EncryptMessage(encryptionKey, localDescJSON)
	if err != nil {
		log.Fatal("Error encrypting local description:", err)
	}

	// Send the encrypted local description
	msgLen := uint32(len(encryptedLocalDesc))
	lenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBytes, msgLen)

	_, err = conn.Write(lenBytes)
	if err != nil {
		log.Fatal("Error sending message length:", err)
	}

	_, err = conn.Write(encryptedLocalDesc)
	if err != nil {
		log.Fatal("Error sending encrypted local description:", err)
	}
}

func listenForHostLocalDescription(peerConnection *pionWebRTC.PeerConnection, encryptionKey []byte) (pionWebRTC.SessionDescription, error) {
	listener, err := net.Listen("tcp", ":9001")
	if err != nil {
		log.Fatal("Error listening for host local description:", err)
	}
	defer listener.Close()

	conn, err := listener.Accept()
	if err != nil {
		log.Fatal("Error accepting host local description:", err)
	}
	defer conn.Close()

	for {
		// Receive the host's local description
		// The message will be read from the connection
		// The message will be decrypted with the shared encryption key and unmarshalled into a SessionDescription
		sessionDescription, err := webrtc.ReceiveEncryptedMessage(conn, encryptionKey)
		if err != nil {
			log.Warn("Error receiving host local description:", err)
			log.Warn("Attempting to receive again")
		} else {
			// If the session description was successfully received, send ours back
			serverLocalDescription(conn, encryptionKey, peerConnection)
			return sessionDescription, nil
		}
	}
	
}

func serverIncomingTrack(track *pionWebRTC.TrackRemote) {
	for {
		// Incoming packets are h264 NewTrackLocalStaticSample packets
		videoDevice, err := gocv.VideoWriterFile("/dev/video0", "MJPG", 60, 1920, 1080, true)
		if err != nil {
			log.Fatal("Error opening video device:", err)
		}
		defer videoDevice.Close()

		for {
			packet, _, err := track.ReadRTP()
			if err != nil {
				log.Warn("Error reading RTP packet:", err)
				break
			}

			// decode into image
			img, err := gocv.IMDecode(packet.Payload, gocv.IMReadColor)
			if err != nil {
				log.Warn("Error decoding image:", err)
				break
			}
			defer img.Close()

			// write to video device
			videoDevice.Write(img)
		}
	}
}

func mpegtsToTrack(track *pionWebRTC.TrackLocalStaticSample) {
	// MPEG-TS stream at 127.0.0.1:1234
	stream, err := gocv.OpenVideoCapture("udp://127.0.0.1:1234")
	if err != nil {
		log.Fatal("Error opening video stream:", err)
	}
	defer stream.Close()

	img := gocv.NewMat()
	defer img.Close()

	for {
		stream.Read(&img)
		if img.Empty() {
			continue
		}

		// encode image
		buf, err := gocv.IMEncode(".jpg", img)
		if err != nil {
			log.Warn("Error encoding image:", err)
			continue
		}

		// write to track
		track.WriteSample(media.Sample{Data: buf.GetBytes()})
	}
}

func main() {
	serverPrivateKey, serverPublicKey, err := security.GenerateDHKeyPair()
	if err != nil {
		log.Fatal("Error generating public/private keys on server", err)
	}
	log.Infof("Generated keys, Public Key Length: %d", len(serverPublicKey))

	// Write the server public key to a file
	if len(serverPublicKey) == 0 {
		log.Warn("Server public key is of length 0, THIS SHOULD NEVER HAPPEN, check the GenerateDHKeyPair function")
	}

	log.Info("Have server public key, writing to file")
	err = os.WriteFile(config.ServerPublicKeyFile, serverPublicKey, config.FilePermissions) //* Why is this empty?
	if err != nil {
		log.Fatal("Error writing server public key to file", err)
	}

	fi, err := os.Stat(config.ServerPublicKeyFile)
	if err != nil {
		log.Fatal("Error stating file", err)
	}
	log.Infof("Wrote %d bytes to %s", fi.Size(), config.ServerPublicKeyFile)

	hostPublicKey, err := os.ReadFile(config.HostPublicKeyFile)
	if err != nil {
		log.Fatal("Error opening host public key file", err)
	}

	sharedSecret, err := security.ComputeSharedSecret(serverPrivateKey, hostPublicKey)
	if err != nil {
		log.Fatal("Error computing shared secret", err)
	}

	encryptionKey, err := security.DeriveEncryptionKey(sharedSecret)
	if err != nil {
		log.Fatal("Error deriving encryption key", err)
	}

	// Create a WebRTC peer connection
	log.Info("Establishing WebRTC connection")
	peerConnection, err := webrtc.CreatePeerConnection()
	if err != nil {
		log.Fatal("Error creating peer connection", err)
	}

	// Listen for the host's local description
	log.Info("Listening for host local description")
	sessionDescription, err := listenForHostLocalDescription(peerConnection, encryptionKey)
	// If the session description was successfully received, set it on the peer connection
	if err == nil {
		err = peerConnection.SetRemoteDescription(sessionDescription)
		if err != nil {
			log.Warn("Error setting remote description:", err)
			log.Warn("Attempting to receive again")
		}
	}

	// Handle incoming tracks
	log.Info("Waiting for camera video")
	peerConnection.OnTrack(func(track *pionWebRTC.TrackRemote, receiver *pionWebRTC.RTPReceiver) {
		go serverIncomingTrack(track)
	})

	// Create an outgoing track
	log.Info("Creating outgoing track")
	track, err := pionWebRTC.NewTrackLocalStaticSample(pionWebRTC.RTPCodecCapability{MimeType: "video/h264"}, "video", "pion")
	if err != nil {
		log.Fatal("Error creating outgoing track:", err)
	}

	// Add the outgoing track to the peer connection
	_, err = peerConnection.AddTrack(track)
	if err != nil {
		log.Fatal("Error adding outgoing track to peer connection:", err)
	}

	go mpegtsToTrack(track)

	// Block forever
	select {}
}
