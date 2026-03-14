#!/bin/bash
echo "Starting Deft Panel Backend in DEV mode..."

# Force kill any process using the ports to ensure a clean start
sudo fuser -k 3000/tcp || true
sudo fuser -k 50051/tcp || true

export DEV=true
export DEFT_JWT_SECRET="a-very-secret-key-that-is-long-enough"
export DEFT_INSECURE_COOKIES="true"

# Use sudo -E to preserve the environment variables for the root user
sudo -E /home/lucasi/sdk/go1.26.1/bin/go run ./cmd/deft-panel/
