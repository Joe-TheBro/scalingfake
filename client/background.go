package main

import (
	"time"

	"github.com/Joe-TheBro/scalingfake/shared/config"
	"github.com/Joe-TheBro/scalingfake/shared/utils"
	"github.com/charmbracelet/log"
)

func background_main() {
	// Allocate a VM //! fix this if you want to use it
	// log.Info("Allocating VM")
	// publicIP := allocateVM()
	// if *publicIP.Properties.IPAddress == "" {
	// 	log.Fatal("Error allocating VM, no public IP address")
	// }

	//generate ssh keypair //! THIS IS DONE ALREADY IN azure.go
	// log.Info("Generating SSH key pair...")
	// err = security.GenerateSSHKey()
	// if err != nil {
	// 	log.Fatal("Error generating SSH key pair", err)
	// }

	var err error
	// var retry int = 0
	

	// // Connect to the VM
	// log.Info("Connecting to VM via SSH...")
	// normalctxSSH := &utils.SSHContext{
	// 	Host:           UIIPAddress, //! publicIP.Properties.IPAddress,
	// 	Port:           config.SSHPort,
	// 	Username:       config.SSHUsername,
	// 	PrivateKeyPath: config.SSHPrivateKeyPath,
	// 	SSHClient:      nil,
	// }

	
	// for {
	// 	normalctxSSH.SSHClient, err = utils.ConnectSSH(normalctxSSH)
	// 	if err != nil {
	// 		if retry < config.MaxSSHRetries {
	// 			log.Warn("Error connecting to VM via SSH, retrying")
	// 			retry++
	// 			continue
	// 		}
	// 		log.Fatal("Failed connecting to VM via SSH", err)
	// 	}
	// 	break
	// }

	// // setup server
	// log.Info("Setting up server...")
	// err = utils.SetupServer(normalctxSSH)
	// if err != nil {
	// 	log.Fatal("Error setting up server...", err)
	// }

	log.Info("Server is setting up...")
	log.Info("Attempting to connect to SSH signaling server...")
	signalingctxSSH := &utils.SSHContext{
		Host:           UIIPAddress, //! publicIP.Properties.IPAddress,
		Port:           2222,
		Username:       config.SSHUsername,
		PrivateKeyPath: config.SSHPrivateKeyPath,
		SSHClient:      nil,
	}

	for {
		signalingctxSSH.SSHClient, err = utils.ConnectSSH(signalingctxSSH)
		if err != nil {
			log.Warn("Unable to connect to SSH signaling server (this likely means the server is still being setup), retrying in 5 seconds...")
			time.Sleep(5 * time.Second)
			continue
		}
		break
	}

	startWebrtcClient(signalingctxSSH)
}
