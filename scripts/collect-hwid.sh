#!/bin/sh
# Собирает аппаратные идентификаторы этого хоста для лицензии PM Dashboard.
#
# docker-compose сам запускает скрипт перед каждым стартом backend (служба
# "hwid"), поэтому обычно делать вручную ничего не нужно.
#
# Скрипт пишет /etc/pmdashboard/hwid-source; каталог монтируется в контейнер
# backend только для чтения. Приложение читает идентификаторы из этого
# файла: само оно железо не опрашивает, и его контейнеру не нужны лишние
# привилегии.
#
# Собираются два идентификатора; лицензия действует, пока не изменился хотя
# бы один из них:
#   machine_id    /etc/machine-id — переживает замену платы
#   board_serial  серийный номер материнской платы — переживает переустановку ОС
#
# Если экран лицензии сообщает, что аппаратный отпечаток недоступен,
# смотрите "docker compose logs hwid". Там, где собрать его автоматически
# нельзя (например, Docker без root не может прочитать серийный номер
# платы), запустите скрипт на ХОСТЕ от root и перезапустите backend:
#
#     sudo sh scripts/collect-hwid.sh

set -eu

TARGET="${1:-/etc/pmdashboard/hwid-source}"

if [ "$(id -u)" -ne 0 ]; then
    echo "Run this script as root: the board serial number is readable by root only." >&2
    exit 1
fi

# Плата без настоящего серийного номера сообщает заглушку; заглушка
# одинакова на тысячах машин и не должна стать привязкой лицензии.
is_placeholder() {
    value=$(printf '%s' "$1" | tr '[:upper:]' '[:lower:]')
    case "$value" in
        ""|*"to be filled"*|*"o.e.m"*|*"oem"*|*"not specified"*|*"not applicable"*|*"default string"*|*"system serial"*|none|unknown|n/a)
            return 0 ;;
    esac
    # Одни нули или одни "x"
    stripped=$(printf '%s' "$value" | tr -d '0x')
    [ -z "$stripped" ]
}

# В контейнере machine-id хоста смонтирован по другому пути
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
# Docker создаёт каталог на месте монтируемого файла, которого не было
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
