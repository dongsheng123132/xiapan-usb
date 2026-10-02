#!/bin/sh
set -eu
task_root=$(mktemp -d /tmp/xiapan-maintenance.XXXXXX)
exe="/mnt/c/1new-code/虾盘-ai优盘精灵版u盘版本/dist/linux-x64/xiapan"
"$exe" action run system.inspect --root "$task_root" --json --no-input
"$exe" action run report.create --root "$task_root" --json --no-input
"$exe" action run reports.list --root "$task_root" --json --no-input
