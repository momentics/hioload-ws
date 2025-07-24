#!/usr/bin/env bash
# ================================================================
# create_aliases_linux.sh
#
# Creates N loopback IP aliases on Linux.
# Usage: ./create_aliases_linux.sh <num_aliases>
# Example: ./create_aliases_linux.sh 128
#
# This script uses `ip` command to add addresses 127.0.0.2..127.0.0.<N+1> on lo.
# Requires root privileges.
# ================================================================

set -euo pipefail

if [ $# -ne 1 ]; then
  echo "Usage: $0 <num_aliases>"
  exit 1
fi

NUM="$1"
BASE="127.0.0"
DEV="lo"

# Ensure we have sufficient privileges
if [ "$(id -u)" -ne 0 ]; then
  echo "This script must be run as root"
  exit 1
fi

# Loop to add aliases
for i in $(seq 2 "$((NUM + 1))"); do
  IP="${BASE}.${i}"
  echo "Adding alias ${IP}"
  ip addr add "${IP}/8" dev "${DEV}"
done

echo "Added $NUM aliases on ${DEV}"
