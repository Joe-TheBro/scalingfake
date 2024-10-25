package main

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"net"
	"os"
	"strings"

	"golang.org/x/crypto/curve25519"
)

// generateDHKeyPair generates a Curve25519 key pair (private and public keys)
func generateDHKeyPair() ([]byte, []byte, error) {
	privateKey := make([]byte, 32)
	_, err := rand.Read(privateKey)
	if err != nil {
		return nil, nil, err
	}
	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, publicKey, nil
}

// Function definition removed to avoid redeclaration error

// handleConnection processes incoming messages and responds with either 'pong' or the server's public key
func handleConnection(conn net.Conn, serverPublicKey []byte) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from connection:", err)
			return
		}

		message = strings.TrimSpace(message)
		message = strings.TrimSuffix(message, "\n")

		if message == "ping" {
			_, err := conn.Write([]byte("pong\n"))
			if err != nil {
				fmt.Println("Error writing pong:", err)
				return
			}
		} else if message == "publickey" {
			_, err := conn.Write([]byte(fmt.Sprintf("key:%s\n", string(serverPublicKey))))
			if err != nil {
				fmt.Println("Error sending public key:", err)
				return
			}
		} else {
			fmt.Println("Unknown message received:", message)
		}
	}
}

func main() {
	// Generate server's own key pair
	_, serverPublicKey, err := generateDHKeyPair()
	if err != nil {
		panic(err)
	}

	// For demo purposes, let's assume the host public key is generated here as well
	hostPublicKey, err := os.ReadFile("hostPublicKey.bin")
	if err != nil {
		panic(err)
	}

	// Print the public keys
	fmt.Println("Host public key:", string(hostPublicKey))
	fmt.Println("Server public key:", string(serverPublicKey))

	// Start listening on TCP port 9001
	listener, err := net.Listen("tcp", "localhost:9001")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Server listening on port 9001")

	for {
		// Accept incoming connection
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		// Handle the connection in a new goroutine
		go handleConnection(conn, serverPublicKey)
	}
}
