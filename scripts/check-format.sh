#!/bin/sh
# Claude Code PostToolUse Hook：Edit/Write 之后检查 cmd/ 与 internal/ 下的 Go 文件是否已 gofmt。
# 只是即时反馈，不替代 make check；也不覆盖通过 Bash 产生的改动。
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$(dirname "$script_dir")"

files=$(find cmd internal -type f -name '*.go' 2>/dev/null || true)
[ -z "$files" ] && exit 0

unformatted=$(gofmt -l $files)
if [ -n "$unformatted" ]; then
    echo "gofmt needed:" >&2
    echo "$unformatted" >&2
    exit 2
fi
