#!/usr/bin/env bash

if [ "$EUID" -ne 0 ]; then
  echo "Please run as root"
  exit 1
fi

echo "Updating system and installing generic kernel..."
apt update
apt install -y linux-generic initramfs-tools curl
curl -LsSf https://astral.sh/uv/install.sh | sh

export PATH="$PATH:$(python3 -m site --user-base)/bin"
hash -r
uv tool install --force /root/grubmod-0.9.1-py3-none-any.whl

echo "Installing Azure Linux Agent..."
apt install -y walinuxagent

echo "Regenerating initramfs to include Hyper-V drivers..."
KERNEL_VERSION=$(ls /boot/vmlinuz-* | grep -oP '(?<=vmlinuz-).*generic' | sort -V | tail -n 1)
if [ -z "$KERNEL_VERSION" ]; then
  echo "Error: No generic kernel found in /boot."
  exit 1
fi

# dracut --force --add-drivers "hv_vmbus hv_storvsc hv_netvsc" /boot/initramfs-${KERNEL_VERSION}.img
update-initramfs -u -k ${KERNEL_VERSION}


# Get the PARTUUID of the root partition
ROOT_PARTUUID=$(blkid -s PARTUUID -o value "$(findmnt -n / -o SOURCE)")

# Check if the PARTUUID was successfully retrieved
if [ -z "$ROOT_PARTUUID" ]; then
  echo "Error: Unable to determine PARTUUID for the root partition."
  exit 1
fi

# Update the GRUB_DEFAULT in /etc/default/grub
echo "Updating GRUB configuration..."
sed -i 's|^GRUB_DEFAULT=.*|GRUB_DEFAULT="Advanced options for Ubuntu>Ubuntu, with Linux '"${KERNEL_VERSION}"'"|' /etc/default/grub

# Update the GRUB_CMDLINE_LINUX entry in /etc/default/grub
sed -i "s|^GRUB_CMDLINE_LINUX=.*|GRUB_CMDLINE_LINUX=\"root=PARTUUID=$ROOT_PARTUUID\"|" /etc/default/grub

echo "Updating GRUB bootloader..."
update-grub

uvx grubmod --kernel-version $KERNEL_VERSION


echo "Creating phase2 service..."

SCRIPT_PATH="/root/phase2.sh"
SERVICE_PATH="/etc/systemd/system/phase2.service"

# Ensure the script exists
if [ ! -f "$SCRIPT_PATH" ]; then
  echo "Error: $SCRIPT_PATH does not exist."
  exit 1
fi

# Create the service file
cat <<EOF | tee "$SERVICE_PATH" > /dev/null
[Unit]
Description=Phase 2 Service - Run After Boot
After=multi-user.target

[Service]
ExecStart=$SCRIPT_PATH
Type=idle

[Install]
WantedBy=multi-user.target
EOF

chmod 644 "$SERVICE_PATH"
systemctl daemon-reload
systemctl enable phase2.service
systemctl start phase2.service

chmod +x "$SCRIPT_PATH"

echo "Setup complete. Rebooting now..."
reboot now
