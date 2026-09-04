#!/bin/sh
set -eu

mkdir -p /var/lib/dsh/data/workspace
printf '%s\n' incompatible-phase3-write > /var/lib/dsh/data/workspace/incompatible-marker
sleep 10
exit 42
