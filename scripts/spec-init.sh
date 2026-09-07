#!/bin/sh
# 用法：make spec-init VERSION=x.y.z
# 前提：Specs/requirements/x.y.z/需求.md 已由人工创建。只创建 Specs/technical/x.y.z/技术方案.md，不覆盖已有文件。
set -eu

script_dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$(dirname "$script_dir")"

V="${1:-}"
case "$V" in
    *[!0-9.]*|"") echo "用法: make spec-init VERSION=x.y.z（语义化三段版本号）" >&2; exit 2 ;;
esac
echo "$V" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || { echo "版本号必须是 x.y.z 形式: $V" >&2; exit 2; }

REQ="Specs/requirements/$V/需求.md"
PLAN_DIR="Specs/technical/$V"
PLAN="$PLAN_DIR/技术方案.md"
TEMPLATE="Specs/technical/技术方案模版.md"

[ -f "$REQ" ] || { echo "人工需求不存在: $REQ（先由人工从 需求模版.md 复制并填写）" >&2; exit 1; }
[ -f "$TEMPLATE" ] || { echo "缺少模版: $TEMPLATE" >&2; exit 1; }
[ -e "$PLAN" ] && { echo "已存在，不覆盖: $PLAN" >&2; exit 1; }

mkdir -p "$PLAN_DIR"
sed "s/{version}/$V/g" "$TEMPLATE" > "$PLAN"
echo "created: $PLAN"
echo "下一步：按模版各节填写技术方案，交用户确认后再编码；make check 会运行 spec-check 核对 Feature ID 与交付状态。"
