package config

import "os"

// Configuration constants and parameters as package-level variables
var (
	RTMPServerURL       = "rtmp://localhost:1935/live/"
	ServerPublicKeyFile = "serverPublicKey.bin"
	// HostPrivateKeyFile string // unused
	HostPublicKeyFile = "hostPublicKey.bin"
	SSHPort           = 22
	SSHUsername       = "overlord"
	SSHPrivateKeyPath = "id_rsa"
	SSHPublicKeyPath  = "id_rsa.pub"
	MaxSSHRetries     = 10
	FilePermissions   = os.FileMode(0666)
	SetupScriptFile   = "setup.sh"
	CameraIndex       = 0
	ServerBinaryPath  = "server"
	DataDir           = "data"
)
