package main

import (
	"fmt"
	"net"
	"os"

	"github.com/pion/webrtc/v3"
)

// Config struct to hold configurable constants and parameters
type Config struct {
	RTMPServerURL       string
	ServerPublicKeyFile string
	// HostPrivateKeyFile string // unused
	HostPublicKeyFile string
	SSHPort           int
	SSHUsername       string
	SSHPrivateKeyPath string
	SSHPublicKeyPath  string
	PrivateKeyPath    string
	MaxSSHRetries     int
	FilePermissions   os.FileMode
	SetupScriptFile   string
	CameraIndex       int
}

// InitializeConfig creates and returns the global configuration instance
func InitializeConfig() Config {
	return Config{
		RTMPServerURL:       "rtmp://localhost:1935/live/",
		ServerPublicKeyFile: "serverPublicKey.bin",
		// HostPrivateKeyFile: "hostPrivateKey.bin", // unused
		HostPublicKeyFile: "hostPublicKey.bin",
		SSHPort:           22,
		SSHUsername:       "overlord",
		SSHPrivateKeyPath: "id_rsa",
		SSHPublicKeyPath:  "id_rsa.pub",
		MaxSSHRetries:     10,
		FilePermissions:   0666,
		SetupScriptFile:   "setup.sh",
		CameraIndex:       0,
	}
}

// Global configuration instance
var config Config

func init() {
	config = InitializeConfig()
}

// TODO: make the printing prettier
func background_main() {
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
	err = os.WriteFile(config.HostPublicKeyFile, hostPublicKey, config.FilePermissions)
	if err != nil {
		fmt.Println("Error writing host public key to file")
		panic(err)
	}
	defer os.Remove(config.HostPublicKeyFile)

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
		Port:           config.SSHPort,
		Username:       config.SSHUsername,
		PrivateKeyPath: config.PrivateKeyPath,
		SSHClient:      nil,
	}

	var retry int = 0
	for {
		ctxSSH.SSHClient, err = connectSSH(ctxSSH)
		if err != nil {
			if retry < config.MaxSSHRetries {
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
	serverPublicKey, err := os.ReadFile(config.ServerPublicKeyFile)
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
	conn, err := net.Dial("tcp", *publicIP.Properties.IPAddress+":9001")
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
