package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"

	"github.com/Joe-TheBro/scalingfake/shared/config"
	"github.com/charmbracelet/log"
)

type rtmpResult struct {
	url string
	err error
}

func generateStreamKey() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", errors.New("failed to generate stream key: " + err.Error())
	}
	return hex.EncodeToString(bytes), nil
}

func startRTMPServer(data <-chan []byte) <-chan rtmpResult {
	resultCh := make(chan rtmpResult)

	go func() {
		defer close(resultCh)

		// Generate a stream key
		streamKey, err := generateStreamKey()
		if err != nil {
			resultCh <- rtmpResult{"", err}
			return
		}

		// Full RTMP URL
		rtmpURL := fmt.Sprintf(config.RTMPServerURL+"%v", streamKey)
		log.Infof("Generated URL: %v", rtmpURL)

		// Setup FFmpeg command
		cmd := exec.Command("ffmpeg",
			"-f", "rawvideo",
			"-pixel_format", "bgr24",
			"-video_size", "1920x1080",
			"-framerate", "60",
			"-i", "pipe:0",
			"-f", "flv",
			"-vcodec", "libx264",
			"-preset", "ultrafast",
			"-pix_fmt", "yuv420p",
			rtmpURL)

		stdin, err := cmd.StdinPipe()
		if err != nil {
			resultCh <- rtmpResult{"", err}
			return
		}

		// Start the command
		if err := cmd.Start(); err != nil {
			resultCh <- rtmpResult{"", err}
			return
		}

		// Start reading from data channel and writing to FFmpeg stdin
		go func() {
			for imgBytes := range data {
				_, err := stdin.Write(imgBytes)
				if err != nil {
					log.Fatal("Error writing to ffmpeg stdin:", err)
					break
				}
			}
			stdin.Close() // Close when channel closes
		}()

		// Wait for the FFmpeg process to complete
		if err := cmd.Wait(); err != nil {
			resultCh <- rtmpResult{"", err}
			return
		}

		resultCh <- rtmpResult{rtmpURL, nil}
	}()

	return resultCh
}
