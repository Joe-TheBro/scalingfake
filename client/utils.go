package main

import (
	"fmt"
	"net"
	"os"
	"time"

	scp "github.com/bramvdbogaerde/go-scp"
	"golang.org/x/crypto/ssh"
)

// Write pump that serializes writes to the connection
func writePump(conn net.Conn, sendChan <-chan string, done chan struct{}) {
	for {
		select {
		case <-done:
			return
		case msg := <-sendChan:
			_, err := conn.Write([]byte(msg))
			if err != nil {
				fmt.Println("Error writing to connection:", err)
				return
			}
		}
	}
}

// Function that continuously sends "ping\n"
func continuouslyPing(sendChan chan<- string, done chan struct{}) {
	ticker := time.NewTicker(1 * time.Second) // Send ping every 1 second
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			sendChan <- "ping\n"
		}
	}
}

func uploadFiles(ip, username, password, localFilePath, remoteFilePath string) error {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", ip), config)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	file, err := os.Open(localFilePath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer file.Close()

	err = scp.Copy(file, remoteFilePath, session)
	if err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}

	return nil
}

func runRemoteShellScript(ip, username, password, scriptPath string) error {
	config := &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	conn, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", ip), config)
	if err != nil {
		return fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	session, err := conn.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %v", err)
	}
	defer session.Close()

	err = session.Run(fmt.Sprintf("sh %s", scriptPath))
	if err != nil {
		return fmt.Errorf("failed to run script: %v", err)
	}

	return nil
}
