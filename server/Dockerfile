FROM nvidia/cuda:11.8.0-cudnn8-runtime-ubuntu20.04
WORKDIR /app
ARG DEBIAN_FRONTEND=noninteractive
ARG QT_QPA_PLATFORM=offscreen
ARG NV_VER
RUN apt update
RUN apt -y install libgl1-mesa-glx libegl1-mesa libxrandr2 libxrandr2 libxss1 libxcursor1 libxcomposite1 libasound2 libxi6 libxtst6 curl ffmpeg git nano gnupg2 libsm6 wget unzip libxcb-icccm4 libxkbcommon-x11-0 libxcb-keysyms1 libxcb-icccm4 libxcb-render0 libxcb-render-util0 libxcb-image0 p7zip-full libx264-dev
RUN apt install -y python3 python3-pip
RUN ln -s /usr/bin/python3 /usr/bin/python
# RUN curl -LsSf https://astral.sh/uv/install.sh | sh
# RUN source ~/.bashrc
# RUN uvx gdown 1QwrnSH-Yq8tkX_H2SVa-Zz7OXbE2hhAM
# RUN 7z x -p ghubsadge DeepFaceLive.7z
# RUN rm deepfacelive.7z

RUN python -m pip install --upgrade pip
RUN python -m pip install onnxruntime-gpu==1.15.1 numpy==1.21.6 h5py numexpr protobuf==3.20.2 opencv-python==4.8.0.74 opencv-contrib-python==4.8.0.74 pyqt6==6.5.1 onnx==1.14.0 torch==1.13.1 torchvision==0.14.1

RUN apt install -y libnvidia-compute-$NV_VER

WORKDIR /app/
COPY DeepFaceLive DeepFaceLive
WORKDIR /app/DeepFaceLive
RUN chmod +x run_noui.sh
CMD ./run_main.sh
