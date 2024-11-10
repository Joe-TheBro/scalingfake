package main

import (
	"net"
	"os"

	"github.com/Joe-TheBro/scalingfake/shared/config"
	"github.com/Joe-TheBro/scalingfake/shared/security"
	"github.com/Joe-TheBro/scalingfake/shared/utils"
	"github.com/Joe-TheBro/scalingfake/shared/webrtc"
	"github.com/charmbracelet/log"
	pionWebRTC "github.com/pion/webrtc/v3"
)

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
	hostPublicKey, hostPrivateKey, err := security.GenerateDHKeyPair()
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
	err = security.GenerateSSHKey()
	if err != nil {
		log.Fatal("Error generating SSH key pair", err)
	}

	// Connect to the VM
	log.Info("Connecting to VM via SSH...")
	ctxSSH := &utils.SSHContext{
		Host:           *publicIP.Properties.IPAddress,
		Port:           config.SSHPort,
		Username:       config.SSHUsername,
		PrivateKeyPath: config.SSHPrivateKeyPath,
		SSHClient:      nil,
	}

	var retry int = 0
	for {
		ctxSSH.SSHClient, err = utils.ConnectSSH(ctxSSH)
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
	err = utils.SetupServer(ctxSSH)
	if err != nil {
		log.Fatal("Error setting up server...", err)
	}

	// Get the server's public key
	log.Info("Getting server public key...")
	err = security.GetServerPublicKey(ctxSSH) //! Possible race condition
	if err != nil {
		log.Fatal("Failed retrieving public key...", err)
	}

	// Compute the shared secret
	log.Info("Computing shared secret")
	serverPublicKey, err := os.ReadFile(config.ServerPublicKeyFile)
	if err != nil {
		log.Fatal("Error reading server public key", err)
	}
	sharedSecret, err := security.ComputeSharedSecret(hostPrivateKey, serverPublicKey)
	if err != nil {
		log.Fatal("Error computing shared secret", err)
	}

	// Derive the encryption key
	log.Info("Deriving encryption key")
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

	// Handle WebRTC signaling
	log.Info("Starting encrypted WebRTC signaling")
	conn, err := net.Dial("tcp", *publicIP.Properties.IPAddress+":9001")
	if err != nil {
		log.Fatal("Error connecting to server:", err)
	}
	defer conn.Close()
	webrtc.HandleWebRTCSignaling(conn, encryptionKey, peerConnection)

	// Send the local camera feed
	log.Info("Sending camera feed")
	if err := webrtc.SendLocalCamera(peerConnection); err != nil {
		log.Fatal("Error sending local camera", err)
	}

	// Handle incoming tracks
	log.Info("Waiting for deepfake video")
	peerConnection.OnTrack(func(track *pionWebRTC.TrackRemote, receiver *pionWebRTC.RTPReceiver) {
		go webrtc.HandleIncomingTrack(track, rtmpData)
	})
}
