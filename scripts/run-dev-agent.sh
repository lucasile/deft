#!/bin/bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

NODE_NAME="${1:-}"
JOIN_TOKEN="${2:-}"
PANEL_URL="${PANEL_URL:-http://localhost:3000}"

if [[ -z "$NODE_NAME" ]]; then
	echo "Usage: $0 <node-name> [64-char-join-token]"
	echo "Optional env: PANEL_URL=http://localhost:3000"
	echo "If no token is provided, the script creates a browser approval link."
	exit 1
fi

if [[ -n "$JOIN_TOKEN" && ! "$JOIN_TOKEN" =~ ^[a-fA-F0-9]{64}$ ]]; then
	echo "Join token must be 64 hex characters. Use the Copy button on the freshly generated token."
	exit 1
fi

AGENT_DIR="$ROOT_DIR/.deft/dev-agents/$NODE_NAME"
CERTS_DIR="$AGENT_DIR/certs"
KEY_PATH="$CERTS_DIR/agent.key"
CSR_PATH="$CERTS_DIR/agent.csr"
REQ_PATH="$AGENT_DIR/join-request.json"
APPROVAL_REQ_PATH="$AGENT_DIR/join-approval-request.json"
STATUS_PATH="$AGENT_DIR/join-status.json"
RESP_PATH="$AGENT_DIR/join-response.json"
CONFIG_PATH="$AGENT_DIR/agent.json"

rm -rf "$AGENT_DIR"
mkdir -p "$CERTS_DIR"

openssl genrsa -out "$KEY_PATH" 2048 >/dev/null 2>&1
openssl req -new -key "$KEY_PATH" -out "$CSR_PATH" -subj "/CN=$NODE_NAME" >/dev/null 2>&1
chmod 600 "$KEY_PATH"

python3 - "$NODE_NAME" "$CSR_PATH" > "$REQ_PATH" <<'PY'
import json
import sys

node_name = sys.argv[1]
csr_path = sys.argv[2]

with open(csr_path, "r", encoding="utf-8") as f:
    csr = f.read()

json.dump({"node_name": node_name, "csr_pem": csr}, sys.stdout)
PY

if [[ -n "$JOIN_TOKEN" ]]; then
	status="$(
		curl -sS \
			-o "$RESP_PATH" \
			-w "%{http_code}" \
			-X POST "$PANEL_URL/api/agent/join" \
			-H "Authorization: Bearer $JOIN_TOKEN" \
			-H "Content-Type: application/json" \
			--data-binary "@$REQ_PATH"
	)"

	if [[ "$status" != "201" ]]; then
		echo "Join failed: HTTP $status"
		cat "$RESP_PATH"
		exit 1
	fi
else
	python3 - "$NODE_NAME" "$CSR_PATH" "$PANEL_URL" > "$APPROVAL_REQ_PATH" <<'PY'
import json
import sys

node_name, csr_path, panel_url = sys.argv[1:]

with open(csr_path, "r", encoding="utf-8") as f:
    csr = f.read()

json.dump({"node_name": node_name, "csr_pem": csr, "panel_url": panel_url}, sys.stdout)
PY

	status="$(
		curl -sS \
			-o "$STATUS_PATH" \
			-w "%{http_code}" \
			-X POST "$PANEL_URL/api/agent/join-requests" \
			-H "Content-Type: application/json" \
			--data-binary "@$APPROVAL_REQ_PATH"
	)"

	if [[ "$status" != "201" ]]; then
		echo "Join request failed: HTTP $status"
		cat "$STATUS_PATH"
		exit 1
	fi

	read -r REQUEST_ID REQUEST_SECRET EXPIRES_AT APPROVAL_URL VERIFICATION_CODE < <(python3 - "$STATUS_PATH" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as f:
    data = json.load(f)

print(data["id"], data["secret"], data["expires_at"], data["approval_url"], data["verification_code"])
PY
	)

	echo
	echo "Open this URL and approve the dev agent:"
	echo
	echo "$APPROVAL_URL"
	echo
	echo "Verification code: $VERIFICATION_CODE"
	echo

	while (( "$(date +%s)" < EXPIRES_AT )); do
		status="$(
			curl -sS \
				-o "$STATUS_PATH" \
				-w "%{http_code}" \
				-H "Authorization: Bearer $REQUEST_SECRET" \
				"$PANEL_URL/api/agent/join-requests/$REQUEST_ID"
		)"
		if [[ "$status" != "200" ]]; then
			echo "Join status failed: HTTP $status"
			cat "$STATUS_PATH"
			exit 1
		fi

		join_status="$(python3 - "$STATUS_PATH" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as f:
    data = json.load(f)

print(data["status"])
PY
		)"

		case "$join_status" in
			approved)
				python3 - "$STATUS_PATH" > "$RESP_PATH" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as f:
    data = json.load(f)

json.dump(data["result"], sys.stdout)
PY
				break
				;;
			pending)
				sleep 3
				;;
			denied|expired)
				echo "Join request $join_status"
				exit 1
				;;
			*)
				echo "Unknown join status: $join_status"
				exit 1
				;;
		esac
	done

	if [[ ! -s "$RESP_PATH" ]]; then
		echo "Join request expired"
		exit 1
	fi
fi

python3 - "$RESP_PATH" "$CERTS_DIR" "$KEY_PATH" "$CONFIG_PATH" <<'PY'
import json
import os
import sys

response_path, certs_dir, key_path, config_path = sys.argv[1:]

with open(response_path, "r", encoding="utf-8") as f:
    result = json.load(f)

ca_path = os.path.join(certs_dir, "ca.crt")
cert_path = os.path.join(certs_dir, "agent.crt")

with open(ca_path, "w", encoding="utf-8") as f:
    f.write(result["ca_cert_pem"])
with open(cert_path, "w", encoding="utf-8") as f:
    f.write(result["cert_pem"])

config = {
    "panel_addr": result["panel_addr"],
    "node_id": result["node_id"],
    "ca_path": ca_path,
    "cert_path": cert_path,
    "key_path": key_path,
}
with open(config_path, "w", encoding="utf-8") as f:
    json.dump(config, f, indent=2)
    f.write("\n")
os.chmod(config_path, 0o600)
PY

echo "Started clean dev agent:"
echo "  name: $NODE_NAME"
echo "  config: $CONFIG_PATH"
echo "  panel: $PANEL_URL"
echo

cd "$ROOT_DIR"
if [[ "${DEFT_DEV_AGENT_USE_BIN:-}" == "true" && -x "$ROOT_DIR/bin/deftd" ]]; then
	exec "$ROOT_DIR/bin/deftd" -config "$CONFIG_PATH"
fi

exec go run ./cmd/deftd -config "$CONFIG_PATH"
