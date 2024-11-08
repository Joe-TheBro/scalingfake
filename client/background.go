package main

import (
	"net"
	"os"

	"github.com/charmbracelet/log"
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
	ServerBinaryPath  string
	DataDir           string
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
		ServerBinaryPath:  "server",
		DataDir:           "data",
	}
}

// Global configuration instance
var config Config

func init() {
	config = InitializeConfig()
}

func background_main() {
	// Channel for RTMP data
	rtmpData := make(chan []byte)
	log.Info("Starting RTMP server...")

	// Start RTMP server asynchronously
	resultCh := startRTMPServer(rtmpData)

	// Process the result asynchronously
	go func() {
		result := <-resultCh
		if result.err != nil {
			log.Fatal("Error starting RTMP server:", result.err)
		} else {
			log.Info("RTMP server started with URL:", result.url)
		}
	}()

	// Generate a key pair for the host
	log.Info("Generating public/private keys on host")
	hostPublicKey, hostPrivateKey, err := generateDHKeyPair()
	if err != nil {
		log.Fatal("Error generating public/private keys on host", err)
	}

	// Write the host public key to a file
	log.Info("Have host public key, writing to file")
	err = os.WriteFile(config.HostPublicKeyFile, hostPublicKey, config.FilePermissions)
	if err != nil {
		log.Fatal("Error writing host public key to file", err)
	}
	defer os.Remove(config.HostPublicKeyFile)

	// Allocate a VM
	log.Info("Allocating VM")
	publicIP := allocateVM()
	if *publicIP.Properties.IPAddress == "" {
		log.Fatal("Error allocating VM, no public IP address")
	}

	//generate ssh keypair
	log.Info("Generating SSH key pair...")
	err = generateSSHKey()
	if err != nil {
		log.Fatal("Error generating SSH key pair", err)
	}

	// Connect to the VM
	log.Info("Connecting to VM via SSH...")
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
				log.Warn("Error connecting to VM via SSH, retrying")
				retry++
				continue
			}
			log.Fatal("Failed connecting to VM via SSH", err)
		}
		break
	}

	// setup server
	log.Info("Setting up server...")
	err = setupServer(ctxSSH)
	if err != nil {
		log.Fatal("Error setting up server...", err)
	}

	// Get the server's public key
	log.Info("Getting server public key...")
	err = getServerPublicKey(ctxSSH) //! Possible race condition
	if err != nil {
		log.Fatal("Failed retrieving public key...", err)
	}

	// Compute the shared secret
	log.Info("Computing shared secret")
	serverPublicKey, err := os.ReadFile(config.ServerPublicKeyFile)
	if err != nil {
		log.Fatal("Error reading server public key", err)
	}
	sharedSecret, err := computeSharedSecret(hostPrivateKey, serverPublicKey)
	if err != nil {
		log.Fatal("Error computing shared secret", err)
	}

	// Derive the encryption key
	log.Info("Deriving encryption key")
	encryptionKey, err := deriveEncryptionKey(sharedSecret)
	if err != nil {
		log.Fatal("Error deriving encryption key", err)
	}

	// Create a WebRTC peer connection
	log.Info("Establishing WebRTC connection")
	peerConnection, err := createPeerConnection()
	if err != nil {
		log.Fatal("Error creating peer connection", err)
	}

	// Handle WebRTC signaling
	log.Info("Starting encrypted WebRTC signaling")
	conn, err := net.Dial("tcp", *publicIP.Properties.IPAddress+":9001")
	if err != nil {
		log.Fatal("Error connecting to server:", err)
	}
	defer conn.Close()
	handleWebRTCSignaling(conn, encryptionKey, peerConnection)

	// Send the local camera feed
	log.Info("Sending camera feed")
	if err := sendLocalCamera(peerConnection); err != nil {
		log.Fatal("Error sending local camera", err)
	}

	// Handle incoming tracks
	log.Info("Waiting for deepfake video")
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		go handleIncomingTrack(track, rtmpData)
	})
}
