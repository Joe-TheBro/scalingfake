package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
)

func generateStreamKey() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.New("failed to generate stream key:" + err.Error())
	}

	streamKey := hex.EncodeToString(bytes)
	return streamKey, nil
}

func startRTMPServer(data <-chan []byte) (string, error) {
	// Generate a stream key
	streamKey, err := generateStreamKey()
	if err != nil {
		fmt.Println("Error generating stream key:", err)
		return "", err
	}

	// Full RTMP URL
	rtmpURL := fmt.Sprintf("rtmp://localhost:1935/live/%s", streamKey)
	fmt.Println("Generated URL:", rtmpURL)

	cmd := exec.Command("ffmpeg",
		"-f", "rawvideo",
		"-pixel_format", "bgr24",
		"-video_size", "640x480",
		"-framerate", "30",
		"-i", "pipe:0",
		"-f", "flv",
		"-vcodec", "libx264",
		"-preset", "ultrafast",
		"-pix_fmt", "yuv420p",
		rtmpURL)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		fmt.Println("Error setting up ffmpeg stdin pipe:", err)
		return "", err
	}

	if err := cmd.Start(); err != nil {
		fmt.Println("Error starting ffmpeg command:", err)
		return "", err
	}

	// Read image bytes from the channel and write to ffmpeg stdin
	for imgBytes := range data {
		_, err = stdin.Write(imgBytes)
		if err != nil {
			fmt.Println("Error writing to ffmpeg stdin:", err)
			break
		}
	}

	// Close the stdin after the channel is closed
	err = stdin.Close()
	if err != nil {
		fmt.Println("Error closing ffmpeg stdin:", err)
		return err
	}

	// Wait for ffmpeg to finish
	if err := cmd.Wait(); err != nil {
		fmt.Println("Error waiting for ffmpeg command:", err)
		return err
	}

	return nil
}
