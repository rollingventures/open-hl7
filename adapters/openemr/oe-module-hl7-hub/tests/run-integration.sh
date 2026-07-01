#!/bin/sh
# Runs the adapter->hub integration test against a locally-built open-hl7 hub.
#   PHPUNIT=/path/to/phpunit  (default: `phpunit` on PATH, else vendor/bin/phpunit)
set -eu

ROOT="$(git -C "$(dirname "$0")" rev-parse --show-toplevel)"
MOD="$ROOT/adapters/openemr/oe-module-hl7-hub"

echo "building hub + test listener..."
go -C "$ROOT" build -o /tmp/oh_hubd_it ./cmd/hubd
go -C "$ROOT" build -o /tmp/oh_listener_it ./cmd/testlistener

/tmp/oh_listener_it -listen :12576 >/tmp/oh_listener_it.log 2>&1 &
LPID=$!
/tmp/oh_hubd_it -mllp-listen :12575 -http :18088 -dest 127.0.0.1:12576 -db /tmp/oh_it.db >/tmp/oh_hubd_it.log 2>&1 &
HPID=$!
trap 'kill "$LPID" "$HPID" 2>/dev/null || true; rm -f /tmp/oh_it.db' EXIT INT TERM

echo "waiting for hub..."
i=0
until curl -fsS http://127.0.0.1:18088/health >/dev/null 2>&1; do
    i=$((i + 1))
    [ "$i" -gt 30 ] && { echo "hub failed to start"; cat /tmp/oh_hubd_it.log; exit 1; }
    sleep 1
done

phpunit="${PHPUNIT:-}"
if [ -z "$phpunit" ]; then
    if [ -x "$MOD/vendor/bin/phpunit" ]; then phpunit="$MOD/vendor/bin/phpunit"; else phpunit="phpunit"; fi
fi

export HUB_BASE="http://127.0.0.1:18088"
cd "$MOD"
echo "running integration suite against $HUB_BASE ..."
"$phpunit" --testsuite integration
