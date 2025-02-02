package main

import (
	"os"

	"github.com/charmbracelet/log"
)

func main() {
	// Read private key
	log.Info("Reading private key")
	privateKey, err := os.ReadFile("/root/.ssh/deepfake-vm_private_key.pem")
	if err != nil {
		log.Fatal("Error reading private key", err)
	}
	log.Info("Private key length: ", len(privateKey))


	// Start webrtc server
	log.Info("Entering webrtc server function")
	StartSshSignalingServer(privateKey)	

	// Block forever
	select {}
}
