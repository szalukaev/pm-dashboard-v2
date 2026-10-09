#!/bin/sh
# Collects the hardware identifiers of this host for the PM Dashboard license.
#
# docker-compose runs it by itself before every start of the backend (the
# "hwid" service), so normally there is nothing to do by hand.
#
# It writes /etc/pmdashboard/hwid-source; the directory is mounted into the
# backend container read-only. The application reads the identifiers from
# this file: it never queries the hardware itself, so its container needs no
# extra privileges.
#
# Two identifiers are collected; the license stays valid while at least one
# of them is unchanged:
#   machine_id    /etc/machine-id — survives a board replacement
#   board_serial  the baseboard serial number — survives an OS reinstall
#
# If the license screen reports that the hardware fingerprint is not
# available, see "docker compose logs hwid". Where it cannot be collected
# automatically (for example, rootless Docker cannot read the board serial
# number), run the script on the HOST as root and restart the backend:
#
#     sudo sh scripts/collect-hwid.sh

set -eu

TARGET="${1:-/etc/pmdashboard/hwid-source}"

if [ "$(id -u)" -ne 0 ]; then
    echo "Run this script as root: the board serial number is readable by root only." >&2
    exit 1
fi

# A board without a real serial number reports a stub; a stub is the same on
# thousands of machines and must not become a license binding.
is_placeholder() {
    value=$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')
    case "$value" in
        ""|*"to be filled"*|*"o.e.m"*|*"oem"*|*"not specified"*|*"not applicable"*|*"default string"*|*"system serial"*|none|unknown|n/a)
            return 0 ;;
    esac
    # Only zeros or only "x"
    stripped=$(printf '%s' "$value" | tr -d '0x')
    [ -z "$stripped" ]
}

# In the container the machine id of the host is mounted at another path
MACHINE_ID_FILE="${MACHINE_ID_FILE:-/etc/machine-id}"

machine_id=""
if [ -f "$MACHINE_ID_FILE" ] && [ -r "$MACHINE_ID_FILE" ]; then
    machine_id=$(tr -d '[:space:]' < "$MACHINE_ID_FILE")
fi

board_serial=""
if command -v dmidecode >/dev/null 2>&1; then
    board_serial=$(dmidecode -s baseboard-serial-number 2>/dev/null | head -n 1 | sed 's/^[[:space:]]*//; s/[[:space:]]*$//' || true)
fi
if [ -z "$board_serial" ] && [ -r /sys/class/dmi/id/board_serial ]; then
    board_serial=$(sed 's/^[[:space:]]*//; s/[[:space:]]*$//' /sys/class/dmi/id/board_serial 2>/dev/null | head -n 1 || true)
fi
if is_placeholder "$board_serial"; then
    if [ -n "$board_serial" ]; then
        echo "WARNING: the board reports a stub instead of a serial number (\"$board_serial\") — it is not used." >&2
    else
        echo "WARNING: the board serial number is not available on this host." >&2
    fi
    board_serial=""
fi

if [ -z "$machine_id" ] && [ -z "$board_serial" ]; then
    echo "ERROR: neither /etc/machine-id nor a board serial number is available; the license cannot be bound to this host." >&2
    exit 1
fi
if [ -z "$machine_id" ]; then
    echo "WARNING: /etc/machine-id is not available; the license will be bound to the board serial number only." >&2
fi
if [ -z "$board_serial" ]; then
    echo "WARNING: the license will be bound to /etc/machine-id only: reinstalling the OS will require a new license." >&2
fi

mkdir -p "$(dirname "$TARGET")"
# Docker creates a directory in place of a mounted file that did not exist
if [ -d "$TARGET" ]; then
    rmdir "$TARGET" 2>/dev/null || {
        echo "ERROR: $TARGET is a directory that is not empty; remove it and run again." >&2
        exit 1
    }
fi
umask 077
{
    echo "# Hardware identifiers of this host for the PM Dashboard license."
    echo "# Written by scripts/collect-hwid.sh; do not edit."
    echo "machine_id=$machine_id"
    echo "board_serial=$board_serial"
} > "$TARGET"
chmod 644 "$TARGET"

echo "Written $TARGET"
