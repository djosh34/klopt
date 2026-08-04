#!/usr/bin/env bash
set -euo pipefail

unit_name=implement-ticket-chain-worker.service
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
root=$(cd -- "$script_dir/../../.." && pwd -P)
worker="$script_dir/worker.sh"
bash_path=$(command -v bash)

: "${PASEO_AGENT_ID:?PASEO_AGENT_ID must be set}"

environment=(
  --setenv="PASEO_AGENT_ID=$PASEO_AGENT_ID"
  --setenv="PATH=$PATH"
)
if [[ -n ${PASEO_HOME:-} ]]; then
  environment+=(--setenv="PASEO_HOME=$PASEO_HOME")
fi

systemd-run \
  --user \
  --unit="$unit_name" \
  --collect \
  --service-type=exec \
  --working-directory="$root" \
  "${environment[@]}" \
  -- "$bash_path" "$worker" "$@"
