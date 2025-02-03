#!/bin/bash

NV_VER=$(modinfo nvidia | grep ^version |awk '{print $2}'|awk -F '.' '{print $1}')

docker build . -t deepfacelive --build-arg NV_VER=$NV_VER
docker run -d --ipc host --gpus all  --volume ./data/:/app/DeepFaceLive/data/ -p 1234:1234 --device=/dev/video0:/dev/video0 --rm deepfacelive
