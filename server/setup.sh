#!/usr/bin/env bash

#! this script is not done, it will break

# Install dependencies
apt update
apt install -y v4l2loopback-dkms nvidia-driver-470 git-all ca-certificates curl
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  tee /etc/apt/sources.list.d/docker.list > /dev/null
apt-get update
apt install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
wget https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2004/x86_64/cuda-ubuntu2004.pin
mv cuda-ubuntu2004.pin /etc/apt/preferences.d/cuda-repository-pin-600
apt-key adv --fetch-keys https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2004/x86_64/7fa2af80.pub
add-apt-repository "deb https://developer.download.nvidia.com/compute/cuda/repos/ubuntu2004/x86_64/ /"
apt update
apt install -y cuda
git clone https://github.com/Joe-TheBro/DeepFaceLive.git
git clone https://github.com/Joe-TheBro/scalingfake.git

# Setup Camera
modprobe v4l2loopback # camera now lives at /dev/video0 /sys/devices/virtual/video4linux

# Setup Deepfakelive
cd DeepFaceLive/build/linux/
./start.sh &

# Start server
cd ../../scalingfake/server/
chmod +x scalingfakeserver
nohup ./scalingfakeserver &