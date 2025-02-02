package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"

	"github.com/Joe-TheBro/scalingfake/shared/config"
	"github.com/Joe-TheBro/scalingfake/shared/utils"
	"github.com/charmbracelet/log"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/ssh"
)

func GenerateDHKeyPair() ([]byte, []byte, error) {
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

func ComputeSharedSecret(privateKey, peerPublicKey []byte) ([]byte, error) {
	sharedSecret, err := curve25519.X25519(privateKey, peerPublicKey)
	if err != nil {
		return nil, err
	}
	return sharedSecret, nil
}

func DeriveEncryptionKey(sharedSecret []byte) ([]byte, error) {
	hash := sha512.New
	hkdf := hkdf.New(hash, sharedSecret, nil, nil)
	key := make([]byte, 32) // 512-bit key for AES-384, documentation is unclear on whether importing 512 is actually 384
	if _, err := io.ReadFull(hkdf, key); err != nil {
		return nil, err
	}
	return key, nil
}

func EncryptMessage(key, plaintext []byte) ([]byte, error) {
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

func DecryptMessage(key, ciphertext []byte) ([]byte, error) {
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

func GetServerPublicKey(ctx *utils.SSHContext) error {
	utils.CopyFile(ctx, "/root/"+config.ServerPublicKeyFile, config.ServerPublicKeyFile)
	log.Info("Received server public key")
	return nil
}

func GenerateSSHKey() error {
	// open private and public key files
	privateKeyFile, err := os.Create(config.SSHPrivateKeyPath)
	if err != nil {
		return fmt.Errorf("failed to create private key file: %v", err)
	}
	defer privateKeyFile.Close()

	publicKeyFile, err := os.Create(config.SSHPublicKeyPath)
	if err != nil {
		return fmt.Errorf("failed to create public key file: %v", err)
	}
	defer publicKeyFile.Close()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %v", err)
	}

	privateKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	}

	err = pem.Encode(privateKeyFile, privateKeyPEM)
	if err != nil {
		return fmt.Errorf("failed to write private key: %v", err)
	}

	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to generate public key: %v", err)
	}

	publicKeyBytes := ssh.MarshalAuthorizedKey(pub)
	_, err = publicKeyFile.Write(publicKeyBytes)
	if err != nil {
		return fmt.Errorf("failed to write public key: %v", err)
	}

	return nil
}
