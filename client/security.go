package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

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

func computeSharedSecret(privateKey, peerPublicKey []byte) ([]byte, error) {
	sharedSecret, err := curve25519.X25519(privateKey, peerPublicKey)
	if err != nil {
		return nil, err
	}
	return sharedSecret, nil
}

func deriveEncryptionKey(sharedSecret []byte) ([]byte, error) {
	hash := sha512.New
	hkdf := hkdf.New(hash, sharedSecret, nil, nil)
	key := make([]byte, 32) // 512-bit key for AES-384, documentation is unclear on whether importing 512 is actually 384
	if _, err := io.ReadFull(hkdf, key); err != nil {
		return nil, err
	}
	return key, nil
}

func encryptMessage(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func decryptMessage(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := aesGCM.NonceSize()
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// Function to handle server messages
func handleServerMessage(c net.Conn, sendChan chan<- string) []byte {
	reader := bufio.NewReader(c)
	for {
		serverMessage, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("Error reading from connection:", err)
			return nil
		}
		serverMessage = strings.TrimSuffix(serverMessage, "\n")
		if serverMessage == "pong" {
			// Send "publickey" request to the server
			sendChan <- "publickey\n"
		} else if strings.HasPrefix(serverMessage, "key:") {
			serverPublicKeyString := strings.TrimPrefix(serverMessage, "key:")
			serverPublicKey := []byte(serverPublicKeyString)
			return serverPublicKey
		}
	}
}

// Start a listener and retrieve the server's public key
func getServerPublicKey() ([]byte, error) {
	var conn net.Conn
	var err error

	for {
		conn, err = net.Dial("tcp", "localhost:9001")
		if err == nil {
			break
		}
		fmt.Println("Connection failed, retrying in 1 second...")
		time.Sleep(1 * time.Second)
	}
	defer conn.Close()

	// Channels for communication and synchronization
	sendChan := make(chan string)
	done := make(chan struct{})

	// Start the write pump
	go writePump(conn, sendChan, done)
	// Start continuously sending "ping\n"
	go continuouslyPing(sendChan, done)

	// Handle incoming server messages
	serverPublicKey := handleServerMessage(conn, sendChan)
	close(done) // Signal goroutines to stop
	if serverPublicKey == nil {
		return nil, errors.New("failed to receive server public key")
	}
	return serverPublicKey, nil
}
