#!/usr/bin/env bash
set -euo pipefail

unit_name=implement-ticket-chain-worker.service

journalctl --user-unit="$unit_name" "$@"
