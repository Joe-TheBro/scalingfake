#!/usr/bin/env bash

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
git clone https://github.com/iperov/DeepFaceLive.git #! fork repo
git clone #!SECRET REPO

# Setup Camera
modprobe v4l2loopback # camera now lives at /dev/video0 /sys/devices/virtual/video4linux

# Setup Deepfakelive
cd DeepFaceLive/build/linux/
NV_LIB=$(locate nvidia.ko |grep $(uname -r) |grep dkms | head -1)
NV_VER=$(modinfo $NV_LIB | grep ^version |awk '{print $2}'|awk -F '.' '{print $1}')
DATA_FOLDER=$(pwd)/data/

docker build . -t deepfacelive --build-arg NV_VER=$NV_VER
docker run -d --ipc host --gpus all -v $DATA_FOLDER:/data/ --device=/dev/video0:/dev/video0 deepfacelive

# Start server
cd #!SECRET REPO
chmod +x scalingfakeserver
nohup ./scalingfakeserver &
cd ../DeepFaceLive
