package main

import (
	"fmt"
	"net"
	"os"

	"github.com/pion/webrtc/v3"
)

// TODO: make the printing prettier
func main() {
	// Channel for RTMP data
	rtmpData := make(chan []byte)
	fmt.Println("Starting RTMP server")

	// Start RTMP server asynchronously
	resultCh := startRTMPServer(rtmpData)

	// Process the result asynchronously
	go func() {
		result := <-resultCh
		if result.err != nil {
			fmt.Println("Error starting RTMP server:", result.err)
		} else {
			fmt.Println("RTMP server started with URL:", result.url)
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
	publicIP := allocateVM()
	if *publicIP.Properties.IPAddress == "" {
		panic("Error allocating VM")
	}

	//generate ssh keypair
	fmt.Println("Generating SSH keypair")
	err = generateSSHKey()

	// Connect to the VM
	fmt.Println("Connecting to VM via SSH")
	ctxSSH := &SSHContext{
		Host:           *publicIP.Properties.IPAddress,
		Port:           22,
		Username:       "overlord",
		PrivateKeyPath: "id_rsa",
		SSHClient:      nil,
	}

	var retry int = 0
	for {
		ctxSSH.SSHClient, err = connectSSH(ctxSSH)
		if err != nil {
			if retry < 10 {
				fmt.Println("Error connecting to VM via SSH, retrying")
				retry++
				continue
			}
			fmt.Println("Error connecting to VM via SSH")
			panic(err)
		}
		break
	}

	// setup server
	fmt.Println("Setting up server")
	err = setupServer(ctxSSH)
	if err != nil {
		fmt.Println("Error setting up server")
		panic(err)
	}

	// Get the server's public key
	fmt.Println("Getting server public key")
	err = getServerPublicKey(ctxSSH)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Compute the shared secret
	fmt.Println("Computing shared secret")
	serverPublicKey, err := os.ReadFile("serverPublicKey.bin")
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
		go handleIncomingTrack(track, rtmpData)
	})
}
