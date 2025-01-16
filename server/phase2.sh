#!/usr/bin/env bash

if [ "$EUID" -ne 0 ]; then
  echo "Please run as root"
  exit 1
fi

cd /root

# Install dependencies
apt update
apt install -y ca-certificates curl p7zip-full gcc libopencv-dev golang-go pkg-config make libtbbmalloc2
curl -LsSf https://astral.sh/uv/install.sh | sh
source $HOME/.local/bin/env
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | \
  tee /etc/apt/sources.list.d/docker.list
apt update
apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

# Build Server
git clone https://github.com/Joe-TheBro/scalingfake.git
cd /root/scalingfake/server
go get
go build -o server
mv server /root/server
cd /root
rm -r ./scalingfake/

# Install CUDA
wget https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2004/x86_64/cuda-keyring_1.1-1_all.deb
dpkg -i cuda-keyring_1.1-1_all.deb
rm cuda-keyring_1.1-1_all.deb
apt update
apt install -y cuda-12-6
apt install -y nvidia-driver-470-server

# Install v4l2loopback
apt install -y v4l2loopback-dkms # install after cuda to avoid kernel version mismatch

# Download DeepFaceLive files from Google Drive
uvx gdown 1QwrnSH-Yq8tkX_H2SVa-Zz7OXbE2hhAM

# Extract files
7z x DeepFaceLive.7z -p"ghubsadge"
rm DeepFaceLive.7z
if [ -f "data.zip" ]; then
  7z x data.zip
  rm data.zip
fi

# Setup Camera
modprobe v4l2loopback # camera now lives at /dev/video0 /sys/devices/virtual/video4linux

# Setup DeepFaceLive
cd /home/overlord/DeepFaceLive/build/linux/
chmod +x ./start.sh
nohup ./start.sh > start.log 2>&1 &

# Start Server
cd /root/
if [ -f "server" ]; then
  chmod +x server
  nohup ./server > server.log 2>&1 &
else
  echo "Server executable not found. Please check your installation."
fi
