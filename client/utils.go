package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bramvdbogaerde/go-scp"
	"golang.org/x/crypto/ssh"
)

type SSHContext struct {
	Host           string
	Port           int
	Username       string
	PrivateKeyPath string
	SSHClient      *ssh.Client
}

// Function that generates a SSH client connectSSH()
// I'll need a context that provides the host, port, and key files
func connectSSH(ctx *SSHContext) (*ssh.Client, error) {
	privateKey, err := os.ReadFile(ctx.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %v", err)
	}

	config := &ssh.ClientConfig{
		User: ctx.Username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", ctx.Host, ctx.Port), config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial: %v", err)
	}

	return client, nil
}

// SCP function that can send/receive files copyFile()
func copyFile(ctx *SSHContext, src, dst string) error {
	sshClient := ctx.SSHClient

	client, err := scp.NewClientBySSH(sshClient)
	if err != nil {
		return fmt.Errorf("failed to create scp client: %v", err)
	}

	defer client.Close()

	// check if the src is local or remote by checking if the file exists
	_, err = os.Stat(src)
	if err == nil {
		// src is local
		file, _ := os.Open(src)
		defer file.Close()

		err = client.CopyFromFile(context.Background(), *file, dst, "0666")
		if err != nil {
			return fmt.Errorf("failed to copy file: %v", err)
		}
	} else {
		// src is remote
		file, err := os.Create(dst)
		if err != nil {
			return fmt.Errorf("failed to create file for copying: %v", err)
		}
		err = client.CopyFromRemote(context.Background(), file, src) // copy from remote to local
		if err != nil {
			return fmt.Errorf("failed to copy file: %v", err)
		}
	}

	return nil
}

// SSH function that will execute a command on the remote server executeCommand()
func executeCommand(ctx *SSHContext, command string) error {
	sshClient := ctx.SSHClient

	session, err := sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	err = session.Run(command)
	if err != nil {
		return fmt.Errorf("failed to run command: %v", err)
	}

	return nil
}

func setupServer(ctx *SSHContext) error {
	// Copy the server binary to the remote server
	err := copyFile(ctx, "server", "server")
	if err != nil {
		return fmt.Errorf("failed to copy server binary: %v", err)
	}

	// Copy the host public key to the remote server
	err = copyFile(ctx, config.HostPublicKeyFile, config.HostPublicKeyFile)
	if err != nil {
		return fmt.Errorf("failed to copy host public key: %v", err)
	}

	// Copy shellscript to the remote server
	err = copyFile(ctx, config.SetupScriptFile, config.SetupScriptFile)
	if err != nil {
		return fmt.Errorf("failed to copy setup script: %v", err)
	}

	// Execute the shellscript on the remote server in background
	err = executeCommand(ctx, "chmod +x "+config.SetupScriptFile+" "+"&& ./"+config.SetupScriptFile+" &")
	if err != nil {
		return fmt.Errorf("failed to execute setup script: %v", err)
	}

	return nil
}
