#!/bin/bash
# install.sh — bootstrap the imggen backend on a fresh Ubuntu host.
#
# Run from the repo root:
#   cd backend && bash deploy/install.sh
#
# Idempotent: re-running upgrades the venv + reloads the systemd unit.
# Resource limits live in deploy/llm-recall-imggen.service:
#   MemoryMax=300M, CPUQuota=50%  (1G / 1C box, shared with Gemini cookie pool)
set -euo pipefail

cd "$(dirname "$0")/.."

# Use the system python3 — Ubuntu 24.04 ships 3.12 by default; 3.10+ is
# fine for FastAPI + Pillow.
if ! command -v python3 >/dev/null; then
  echo "error: python3 not found; install with: sudo apt install -y python3 python3-venv python3-pip"
  exit 1
fi

python3 -m venv .venv
.venv/bin/pip install --upgrade pip
.venv/bin/pip install -r requirements.txt

# systemd --user unit. We deliberately install at user scope, NOT system
# scope, to avoid sudo / pollute /etc on a shared box.
mkdir -p "$HOME/.config/systemd/user"
cp deploy/llm-recall-imggen.service "$HOME/.config/systemd/user/"

# Enable lingering so the service survives logout (one-time).
if command -v loginctl >/dev/null; then
  loginctl enable-linger "$USER" 2>/dev/null || true
fi

systemctl --user daemon-reload
systemctl --user enable --now llm-recall-imggen

echo
echo "OK. Service started."
echo "Verify: curl http://localhost:8001/health"
echo "Open firewall: sudo ufw allow 8001/tcp"
