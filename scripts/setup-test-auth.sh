#!/bin/bash
set -e

# Create directories
sudo mkdir -p /etc/deft/certs

# Create OpenSSL config for SANs
cat <<SAN_EOF | sudo tee /etc/deft/certs/openssl.cnf > /dev/null
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_req
prompt = no
[req_distinguished_name]
CN = localhost
[v3_req]
subjectAltName = @alt_names
[alt_names]
DNS.1 = localhost
IP.1 = 127.0.0.1
SAN_EOF

# Generate CA
echo "Generating CA..."
sudo openssl genrsa -out /etc/deft/certs/ca.key 2048
sudo openssl req -x509 -new -nodes -key /etc/deft/certs/ca.key -sha256 -days 365 -out /etc/deft/certs/ca.crt -subj "/CN=Deft-Test-CA"

# Generate Agent Cert
echo "Generating Agent Cert..."
sudo openssl genrsa -out /etc/deft/certs/agent.key 2048
sudo openssl req -new -key /etc/deft/certs/agent.key -out /etc/deft/certs/agent.csr -subj "/CN=test-node"
sudo openssl x509 -req -in /etc/deft/certs/agent.csr -CA /etc/deft/certs/ca.crt -CAkey /etc/deft/certs/ca.key -CAcreateserial -out /etc/deft/certs/agent.crt -days 365 -sha256

# Generate Panel Cert with SANs
echo "Generating Panel Cert (with SAN)..."
sudo openssl genrsa -out /etc/deft/certs/panel.key 2048
sudo openssl req -new -key /etc/deft/certs/panel.key -out /etc/deft/certs/panel.csr -config /etc/deft/certs/openssl.cnf
sudo openssl x509 -req -in /etc/deft/certs/panel.csr -CA /etc/deft/certs/ca.crt -CAkey /etc/deft/certs/ca.key -CAcreateserial -out /etc/deft/certs/panel.crt -days 365 -sha256 -extfile /etc/deft/certs/openssl.cnf -extensions v3_req

# Create agent.json
echo "Creating /etc/deft/agent.json..."
sudo tee /etc/deft/agent.json > /dev/null <<INNER_EOF
{
    "panel_addr": "localhost:50051",
    "node_id": "local-test-node",
    "ca_path": "/etc/deft/certs/ca.crt",
    "cert_path": "/etc/deft/certs/agent.crt",
    "key_path": "/etc/deft/certs/agent.key"
}
INNER_EOF

# Create panel.json
echo "Creating /etc/deft/panel.json..."
sudo tee /etc/deft/panel.json > /dev/null <<INNER_EOF
{
    "http_port": "3000",
    "grpc_port": "50051",
    "db_path": "panel.db",
    "ca_path": "/etc/deft/certs/ca.crt",
    "cert_path": "/etc/deft/certs/panel.crt",
    "key_path": "/etc/deft/certs/panel.key"
}
INNER_EOF

sudo chmod 600 /etc/deft/certs/agent.key /etc/deft/certs/panel.key
echo "Test environment setup complete!"
