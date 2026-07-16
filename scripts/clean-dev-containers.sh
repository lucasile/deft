#!/bin/bash
set -euo pipefail

NODE_ID="${NODE_ID:-}"

filters=(--filter "label=deft.managed=true")
if [[ -n "$NODE_ID" ]]; then
	filters+=(--filter "label=deft.node_id=$NODE_ID")
fi

mapfile -t containers < <(docker ps -aq "${filters[@]}")

if [[ "${#containers[@]}" -eq 0 ]]; then
	echo "No Deft-managed dev containers found."
	exit 0
fi

echo "Deft-managed containers to remove:"
docker ps -a "${filters[@]}" --format '  {{.ID}}  {{.Names}}  {{.Status}}  node={{.Label "deft.node_id"}}'
echo

if [[ "${YES:-}" != "true" ]]; then
	read -r -p "Remove these containers? [y/N] " answer
	case "$answer" in
		y|Y|yes|YES) ;;
		*) echo "Cancelled."; exit 1 ;;
	esac
fi

docker rm -f "${containers[@]}"
echo "Removed ${#containers[@]} Deft-managed container(s)."
