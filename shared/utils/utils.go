package utils

import (
	"archive/zip"
	"compress/flate"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Joe-TheBro/scalingfake/shared/config"
	"github.com/bramvdbogaerde/go-scp"
	"github.com/charmbracelet/log"
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
func ConnectSSH(ctx *SSHContext) (*ssh.Client, error) {
	privateKey, err := os.ReadFile(ctx.PrivateKeyPath)
	if err != nil {
		log.Fatalf("Error reading private key file: %v", err)
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
		log.Fatalf("Error dialing SSH server: %v", err)
	}

	return client, nil
}

func handleZip(filepath string) error {
	info, err := os.Stat(filepath)
	if err != nil {
		return fmt.Errorf("could not stat file: %w", err)
	}

	if info.IsDir() {
		return zipDirectory(filepath)
	} else if strings.HasSuffix(filepath, ".zip") {
		return unzipArchive(filepath)
	} else {
		return fmt.Errorf("provided path is neither a directory nor a zip archive")
	}
}

func zipDirectory(source string) error {
	zipFileName := filepath.Base(source) + ".zip"
	zipFile, err := os.Create(zipFileName)
	if err != nil {
		return fmt.Errorf("could not create zip file: %w", err)
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	zipWriter.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestCompression)
	})

	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		if info.IsDir() {
			if relPath == "." {
				return nil
			}
			_, err = zipWriter.Create(relPath + "/")
			return err
		}

		zipEntry, err := zipWriter.Create(relPath)
		if err != nil {
			return err
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(zipEntry, file)
		return err
	})

	if err != nil {
		return fmt.Errorf("error zipping directory: %w", err)
	}

	log.Infof("Directory %s successfully zipped as %s\n", source, zipFileName)
	return nil
}

func unzipArchive(zipFilePath string) error {
	reader, err := zip.OpenReader(zipFilePath)
	if err != nil {
		return fmt.Errorf("could not open zip archive: %w", err)
	}
	defer reader.Close()

	destination := strings.TrimSuffix(zipFilePath, ".zip")
	err = os.MkdirAll(destination, 0755)
	if err != nil {
		return fmt.Errorf("could not create output directory: %w", err)
	}

	for _, file := range reader.File {
		outputPath := filepath.Join(destination, file.Name)
		if file.FileInfo().IsDir() {
			err = os.MkdirAll(outputPath, 0755)
			if err != nil {
				return fmt.Errorf("could not create directory %s: %w", outputPath, err)
			}
			continue
		}

		err = os.MkdirAll(filepath.Dir(outputPath), 0755)
		if err != nil {
			return fmt.Errorf("could not create directory for file %s: %w", outputPath, err)
		}

		outputFile, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("could not create file %s: %w", outputPath, err)
		}

		sourceFile, err := file.Open()
		if err != nil {
			outputFile.Close()
			return fmt.Errorf("could not open file in zip archive: %w", err)
		}

		_, err = io.Copy(outputFile, sourceFile)
		outputFile.Close()
		sourceFile.Close()
		if err != nil {
			return fmt.Errorf("could not write file %s: %w", outputPath, err)
		}
	}

	log.Infof("Zip archive %s successfully extracted to %s\n", zipFilePath, destination)
	return nil
}

func CopyFile(ctx *SSHContext, src, dst string) error {
	sshClient := ctx.SSHClient

	client, err := scp.NewClientBySSH(sshClient)
	if err != nil {
		log.Fatalf("failed to create scp client: %v", err)
	}

	defer client.Close()

	// check if the src is local or remote by checking if the file exists
	_, err = os.Stat(src)
	if err == nil {
		// src is local
		// is it a file or a directory?
		if info, err := os.Stat(src); err == nil && info.IsDir() {
			// src is a directory so we zip it
			err = handleZip(src)
			if err != nil {
				log.Fatalf("failed to zip directory: %v", err)
			}

			// copy the zipped file
			file, err := os.Open(strings.TrimSuffix(src,"/") + ".zip")
			if err != nil {
				log.Fatalf("failed to open zipped file: %v", err)
			}
			defer file.Close()

			err = client.CopyFromFile(context.Background(), *file, dst, "0666")
			if err != nil {
				log.Fatalf("failed to copy file: %v", err)
			}
		} else {
			// src is a file
			file, _ := os.Open(src)
			defer file.Close()

			err = client.CopyFromFile(context.Background(), *file, dst, "0666")
			if err != nil {
				log.Fatalf("failed to copy file: %v", err)
			}
		}
	} else {
		//! we only need this function to download a public key from the server, NEVER USE THIS FOR DIRECTORIES
		// src is remote
		file, err := os.Create(dst)
		if err != nil {
			log.Fatalf("failed to create file for copying: %v", err)
		}
		err = client.CopyFromRemote(context.Background(), file, src) // copy from remote to local
		if err != nil {
			log.Fatalf("failed to copy file: %v", err)
		}
	}

	return nil
}

// SSH function that will execute a command on the remote server executeCommand()
func ExecuteCommand(ctx *SSHContext, command string) error {
    sshClient := ctx.SSHClient
    if sshClient == nil {
        return fmt.Errorf("SSH client is not connected")
    }

    session, err := sshClient.NewSession()
    if err != nil {
        log.Errorf("failed to create ssh session: %v", err)
        return err
    }
    defer session.Close()

	log.Infof("Executing command: %s", command)
    err = session.Run(command)
    if err != nil {
        log.Errorf("failed to run command: %v", err)
        return err
    }

    return nil
}

func CleanupClient() {
	//* I realise I should fix the underlying issue with defer statements not being called, but this is a quick fix
	// check for certain files and delete them
	if _, err := os.Stat("data.zip"); err == nil {
		os.Remove("data.zip")
	}

	if _, err := os.Stat(config.HostPublicKeyFile); err == nil {
		os.Remove(config.HostPublicKeyFile)
	}

	if _, err := os.Stat(config.ServerPublicKeyFile); err == nil {
		os.Remove(config.ServerPublicKeyFile)
	}

	if _, err := os.Stat("deepfake-vm_private_key.pem"); err == nil {
		os.Remove("deepfake-vm_private_key.pem")
		// log.Warn("deepfake-vm_private_key.pem not removed")
	}

	if _, err := os.Stat("deepfake-vm_public_key.pub"); err == nil {
		os.Remove("deepfake-vm_public_key.pub")
	}
}

func SetupServer(ctx *SSHContext) error {
	// Copy the server binary to the remote server
	// log.Info("Copying server binary")
	// err := CopyFile(ctx, config.ServerBinaryPath, "/home/overlord/server")
	// if err != nil {
	// 	log.Error("failed to copy server binary: %v", err)
	// 	return err
	// }

	//* we are now pulling from google drive instead of copying the DeepFaceLive directory, upload speed is ~700KB/s
	// log.Info("Copying DeepFaceLive directory")
	// err = CopyFile(ctx, config.DeepFaceLivePath, "/home/overlord/DeepFaceLive.zip")
	// if err != nil {
	// 	log.Error("failed to copy DeepFaceLive directory: %v", err)
	// 	return err
	// }

	log.Info("Copying server public key")
	// Copy the host public key to the remote server
	err := CopyFile(ctx, config.HostPublicKeyFile, "/root/hostPublicKey.bin")
	if err != nil {
		log.Error("failed to copy host public key: %v", err)
		return err
	}

	log.Info("Copying startup scripts")
	// Copy shellscript to the remote server
	err = CopyFile(ctx, config.Phase1ScriptFile, "/root/phase1.sh")
	if err != nil {
		log.Error("failed to copy setup script: %v", err)
		return err
	}

	err = CopyFile(ctx, config.Phase2ScriptFile, "/root/phase2.sh")
	if err != nil {
		log.Error("failed to copy setup script: %v", err)
		return err
	}

	log.Info("Copying grubmod tool")
	err = CopyFile(ctx, config.GrubModWhl, "/root/grubmod-0.9.1-py3-none-any.whl")
	if err != nil {
		log.Error("failed to copy grubmod tool: %v", err)
		return err
	}

	// this directory is small enough to be copied instead of prepared beforehand
	log.Info("Copying data directory")
	//copy data directory to server
	err = CopyFile(ctx, config.DataDir, "/root/data.zip")
	if err != nil {
		log.Error("failed to copy data directory: %v", err)
		return err
	}

	log.Info("Executing setup script")
	// Execute the shellscript on the remote server in background
	// err = ExecuteCommand(ctx, "chmod +x /home/overlord/phase1.sh && sudo nohup /home/overlord/phase1.sh > /home/overlord/phase1.log 2>&1 &")
	err = ExecuteCommand(ctx, "chmod +x /root/phase2.sh && sudo nohup /root/phase2.sh > /root/phase2.log 2>&1 &")
	// command := fmt.Sprintf("chmod +x /home/overlord/phase1.sh && nohup /home/overlord/phase1.sh > /home/overlord/phase1.sh.log 2>&1 &")
	// err = ExecuteCommand(ctx, command)
	if err != nil {
		log.Error("failed to execute setup script: %v", err)
		return err
	}

	return nil
}
