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

func serverIncomingTrack(track *pionWebRTC.TrackRemote, receiver *pionWebRTC.RTPReceiver) {
	for {
		//IMPLEMENT
	}
}

func main() {
	serverPrivateKey, serverPublicKey, err := security.GenerateDHKeyPair()
	if err != nil {
		log.Fatal("Error generating public/private keys on server", err)
	}

	// Write the server public key to a file
	log.Info("Have server public key, writing to file")
	err = os.WriteFile(config.ServerPublicKeyFile, serverPublicKey, config.FilePermissions)

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
		go serverIncomingTrack(track, receiver)
	})
}
