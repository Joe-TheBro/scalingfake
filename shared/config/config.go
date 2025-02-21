package config

import "os"

// Configuration constants and parameters as package-level variables
var (
	RTMPServerURL       = "rtmp://localhost:1935/live/"
	ServerPublicKeyFile = "serverPublicKey.bin"
	// HostPrivateKeyFile string // unused
	HostPublicKeyFile = "hostPublicKey.bin"
	SSHPort           = 22
	SSHUsername       = "root"
	SSHPrivateKeyPath = "deepfake-vm_private_key.pem"
	SSHPublicKeyPath  = "deepfake-vm_public_key.pub"
	MaxSSHRetries     = 10
	FilePermissions   = os.FileMode(0666)
	Phase1ScriptFile  = "./server/phase1.sh"
	Phase2ScriptFile  = "./server/phase2.sh"
	GrubModWhl 	      = "./server/grubmod/dist/grubmod-0.9.1-py3-none-any.whl"
	SetupScriptFile   = "./server/setup.sh"
	CameraIndex       = 0
	ServerBinaryPath  = "./server/server.exe"
	DataDir           = "./data/"
	DeepFaceLivePath  = "./DeepFaceLive/"
	FaceImgPath       = "./face.jpg"
)
