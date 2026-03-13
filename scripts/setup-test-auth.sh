#!/bin/bash
set -e

# 1. Create directories
sudo mkdir -p /etc/deft/certs

# 2. Generate CA
echo "Generating CA..."
sudo openssl genrsa -out /etc/deft/certs/ca.key 2048
sudo openssl req -x509 -new -nodes -key /etc/deft/certs/ca.key -sha256 -days 365 -out /etc/deft/certs/ca.crt -subj "/CN=Deft-Test-CA"

# 3. Generate Agent Cert
echo "Generating Agent Cert..."
sudo openssl genrsa -out /etc/deft/certs/agent.key 2048
sudo openssl req -new -key /etc/deft/certs/agent.key -out /etc/deft/certs/agent.csr -subj "/CN=test-node"
sudo openssl x509 -req -in /etc/deft/certs/agent.csr -CA /etc/deft/certs/ca.crt -CAkey /etc/deft/certs/ca.key -CAcreateserial -out /etc/deft/certs/agent.crt -days 365 -sha256

# 4. Create agent.json
echo "Creating /etc/deft/agent.json..."
sudo tee /etc/deft/agent.json <<INNER_EOF
{
    "panel_addr": "localhost:50051",
    "node_id": "local-test-node",
    "ca_path": "/etc/deft/certs/ca.crt",
    "cert_path": "/etc/deft/certs/agent.crt",
    "key_path": "/etc/deft/certs/agent.key"
}
INNER_EOF

sudo chmod 600 /etc/deft/certs/agent.key
echo "Test environment setup complete!"
