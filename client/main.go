package main

import (
	"fmt"
	"net"
	"os"

	"github.com/pion/webrtc/v3"
)

// TODO: make the printing prettier
func main() {
	//Generate thee stream key
	streamKey, err := generateStreamKey()
	if err != nil {
		fmt.Println("Error generating stream key:", err)
		return
	}

	// Full RTMP URL
	rtmpURL := fmt.Sprintf("rtmp://localhost:1935/live/%s", streamKey)
	fmt.Println("Generated URL:", rtmpURL)

	
	// Start RTMP server
	data := make(chan []byte)
	fmt.Println("Starting RTMP server")
	go func() {
		err = startRTMPServer(data, rtmpURL)
		if err != nil {
			fmt.Println("Error starting RTMP server:", err)
		}
		
	}() 

	// Generate a key pair for the host
	fmt.Println("Generating public/private keys on host")
	hostPublicKey, hostPrivateKey, err := generateDHKeyPair()
	if err != nil {
		fmt.Println("Error generating public/private keys on host")
		panic(err)
	}

	// Write the host public key to a file
	fmt.Println("Have host public key, writing to file")
	err = os.WriteFile("hostPublicKey.bin", hostPublicKey, 0666)
	if err != nil {
		fmt.Println("Error writing host public key to file")
		panic(err)
	}
	defer os.Remove("hostPublicKey.bin")

	// Allocate a VM
	fmt.Println("Allocating VM")
	allocateVM()

	//TODO: upload shell script and public key to VM

	// Get the server's public key
	fmt.Println("Getting server public key")
	serverPublicKey, err := getServerPublicKey() // TODO: rewrite to use SCP
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Received server public key")

	// Compute the shared secret
	fmt.Println("Computing shared secret")
	sharedSecret, err := computeSharedSecret(hostPrivateKey, serverPublicKey)
	if err != nil {
		fmt.Println("Error computing shared secret")
		panic(err)
	}

	// Derive the encryption key
	fmt.Println("Deriving encryption key")
	encryptionKey, err := deriveEncryptionKey(sharedSecret)
	if err != nil {
		fmt.Println("Error deriving encryption key")
		panic(err)
	}

	// Create a WebRTC peer connection
	fmt.Println("Establishing WebRTC connection")
	peerConnection, err := createPeerConnection()
	if err != nil {
		fmt.Println("Error creating peer connection")
		panic(err)
	}

	// Handle WebRTC signaling
	fmt.Println("Starting encrypted WebRTC signaling")
	conn, err := net.Dial("tcp", "localhost:9001")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		panic(err)
	}
	defer conn.Close()
	handleWebRTCSignaling(conn, encryptionKey, peerConnection)

	// Send the local camera feed
	fmt.Println("Sending camera feed")
	if err := sendLocalCamera(peerConnection); err != nil {
		fmt.Println("Error sending local camera")
		panic(err)
	}

	// Handle incoming tracks
	fmt.Println("Waiting for deepfake video")
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		go handleIncomingTrack(track, data)
	})
}
