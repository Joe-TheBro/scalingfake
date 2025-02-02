package main

import (
	"os"

	"github.com/charmbracelet/log"
)

func main() {
	// Read private key
	privateKey, err := os.ReadFile("/root/.ssh/authorized_keys")
	if err != nil {
		log.Fatal("Error reading private key", err)
	}

	// Start webrtc server
	StartSshSignalingServer(privateKey)	

	// Block forever
	select {}
}
