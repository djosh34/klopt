#!/usr/bin/env bash
set -euo pipefail

unit_name=implement-ticket-chain-worker.service

systemctl --user stop "$unit_name"
