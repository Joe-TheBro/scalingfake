package main

import (
	"fmt"
	"net"
	"os"

	"github.com/pion/webrtc/v3"
)

func main() {
	// Start RTMP server
	data := make(chan []byte)
	go startRTMPServer(data)

	// Generate a key pair for the host
	hostPublicKey, hostPrivateKey, err := generateDHKeyPair()
	if err != nil {
		fmt.Println("Error generating public/private keys on host")
		panic(err)
	}

	// Write the host public key to a file
	fmt.Println("Host public key:", string(hostPublicKey))
	err = os.WriteFile("hostPublicKey.bin", hostPublicKey, 0666)
	if err != nil {
		fmt.Println("Error writing host public key to file")
		panic(err)
	}
	defer os.Remove("hostPublicKey.bin")

	go allocateVM()

	// Get the server's public key
	serverPublicKey, err := getServerPublicKey()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Received server public key:", string(serverPublicKey))

	// Compute the shared secret
	sharedSecret, err := computeSharedSecret(hostPrivateKey, serverPublicKey)
	if err != nil {
		fmt.Println("Error computing shared secret")
		panic(err)
	}

	// Derive the encryption key
	encryptionKey, err := deriveEncryptionKey(sharedSecret)
	if err != nil {
		fmt.Println("Error deriving encryption key")
		panic(err)
	}

	// Create a WebRTC peer connection
	peerConnection, err := createPeerConnection()
	if err != nil {
		fmt.Println("Error creating peer connection")
		panic(err)
	}

	// Handle WebRTC signaling
	conn, err := net.Dial("tcp", "localhost:9001")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		panic(err)
	}
	defer conn.Close()
	handleWebRTCSignaling(conn, encryptionKey, peerConnection)

	// Send the local camera feed
	if err := sendLocalCamera(peerConnection); err != nil {
		fmt.Println("Error sending local camera")
		panic(err)
	}

	// Handle incoming tracks
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		go handleIncomingTrack(track, data)
	})
}
