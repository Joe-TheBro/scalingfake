package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"gocv.io/x/gocv"
)

func createPeerConnection() (*webrtc.PeerConnection, error) {
	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}
	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		return nil, err
	}
	return peerConnection, nil
}

func sendLocalCamera(peerConnection *webrtc.PeerConnection) error {
	webcam, err := gocv.OpenVideoCapture(0) // Adjust camera index as needed
	if err != nil {
		return err
	}
	defer webcam.Close()

	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{
		MimeType: "video/H264",
	}, "video", "pion")
	if err != nil {
		return err
	}
	if _, err = peerConnection.AddTrack(videoTrack); err != nil {
		return err
	}

	img := gocv.NewMat()
	defer img.Close()

	go func() {
		for {
			if ok := webcam.Read(&img); !ok {
				break
			}
			// Encode frame to RTP packet
			sample, err := encodeFrameToSample(img)
			if err != nil {
				fmt.Println("Error encoding frame:", err)
				break
			}
			videoTrack.WriteSample(sample)
		}
	}()
	return nil
}

func decodeRTPPacketToFrame(packet rtp.Packet) (gocv.Mat, error) {
	img, err := gocv.IMDecode(packet.Payload, gocv.IMReadColor)
	if err != nil {
		return gocv.Mat{}, err
	}
	return img, nil
}

func encodeFrameToSample(img gocv.Mat) (media.Sample, error) {
	encodedFrame, err := gocv.IMEncode(".jpg", img)
	if err != nil {
		return media.Sample{}, err
	}

	return media.Sample{
		Data: encodedFrame.GetBytes(),
	}, nil
}

// handleIncomingTrack sends the decoded image bytes to a channel
func handleIncomingTrack(track *webrtc.TrackRemote, data chan<- []byte) {
	defer close(data) // Ensure the channel is closed when done
	img := gocv.NewMat()
	defer img.Close()

	for {
		packet, _, err := track.ReadRTP()
		if err != nil {
			fmt.Println("Error reading RTP packet:", err)
			break
		}

		frame, err := packet.Marshal()
		if err != nil {
			fmt.Println("Error marshalling packet:", err)
			break
		}

		img, err = gocv.IMDecode(frame, gocv.IMReadColor)
		if err != nil {
			fmt.Println("Error decoding frame:", err)
			break
		}

		// Convert image to bytes and send to channel
		imgBytes := img.ToBytes()
		data <- imgBytes
	}
}

func handleWebRTCSignaling(conn net.Conn, encryptionKey []byte, peerConnection *webrtc.PeerConnection) {
	// Create an offer
	offer, err := peerConnection.CreateOffer(nil)
	if err != nil {
		fmt.Println("Error creating offer:", err)
		return
	}

	// Set the local description
	err = peerConnection.SetLocalDescription(offer)
	if err != nil {
		fmt.Println("Error setting local description:", err)
		return
	}

	// Wait for ICE gathering to complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	<-gatherComplete

	// Get the local description
	localDesc := peerConnection.LocalDescription()

	// Convert the local description to JSON
	localDescJSON, err := json.Marshal(localDesc)
	if err != nil {
		fmt.Println("Error marshalling local description to JSON:", err)
		return
	}

	// Encrypt the local description
	encryptedLocalDesc, err := encryptMessage(encryptionKey, localDescJSON)
	if err != nil {
		fmt.Println("Error encrypting local description:", err)
		return
	}

	// Send the encrypted local description to the server
	// Prefix the message with its length (uint32)
	msgLen := uint32(len(encryptedLocalDesc))
	lenBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBytes, msgLen)

	// Send length and encrypted message
	_, err = conn.Write(lenBytes)
	if err != nil {
		fmt.Println("Error sending message length:", err)
		return
	}

	_, err = conn.Write(encryptedLocalDesc)
	if err != nil {
		fmt.Println("Error sending encrypted local description:", err)
		return
	}

	// Now, receive the encrypted remote description from the server
	// First, read the length
	_, err = io.ReadFull(conn, lenBytes)
	if err != nil {
		fmt.Println("Error reading message length:", err)
		return
	}

	msgLen = binary.BigEndian.Uint32(lenBytes)
	encryptedRemoteDesc := make([]byte, msgLen)
	_, err = io.ReadFull(conn, encryptedRemoteDesc)
	if err != nil {
		fmt.Println("Error reading encrypted remote description:", err)
		return
	}

	// Decrypt the remote description
	remoteDescJSON, err := decryptMessage(encryptionKey, encryptedRemoteDesc)
	if err != nil {
		fmt.Println("Error decrypting remote description:", err)
		return
	}

	// Unmarshal the remote description
	var remoteDesc webrtc.SessionDescription
	err = json.Unmarshal(remoteDescJSON, &remoteDesc)
	if err != nil {
		fmt.Println("Error unmarshalling remote description:", err)
		return
	}

	// Set the remote description
	err = peerConnection.SetRemoteDescription(remoteDesc)
	if err != nil {
		fmt.Println("Error setting remote description:", err)
		return
	}
}
