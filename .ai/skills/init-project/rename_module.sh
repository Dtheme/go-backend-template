#!/bin/bash
#
# rename_module.sh - 从模板复制出新工程并重命名 Go module
#
# 用法: bash .ai/skills/init-project/rename_module.sh <新目录名> <新module路径> [目标父目录]
# 示例: bash .ai/skills/init-project/rename_module.sh order-service github.com/acme/order-service
#       bash .ai/skills/init-project/rename_module.sh order-service github.com/acme/order-service ~/workspace
#
# 目标父目录默认为模板工程的同级目录。原模板工程不受影响。

set -euo pipefail

# BSD sed（macOS）与 GNU sed（Linux）的 -i 语法不同
if sed --version >/dev/null 2>&1; then
    sedi() { sed -i "$@"; }
else
    sedi() { sed -i '' "$@"; }
fi

OLD_MODULE="example.com/go-backend-template"
OLD_NAME="go-backend-template"

if [ $# -lt 2 ]; then
    echo "用法: $0 <新目录名> <新module路径> [目标父目录]"
    exit 1
fi

NEW_NAME="$1"
NEW_MODULE="$2"

if [[ ! "$NEW_NAME" =~ ^[A-Za-z_][A-Za-z0-9_-]*$ ]]; then
    echo "错误: 目录名只能包含字母、数字、下划线和连字符，且不能以数字开头"
    exit 1
fi
if [ "$NEW_MODULE" = "$OLD_MODULE" ]; then
    echo "错误: 新 module 路径不能与模板相同"
    exit 1
fi
if [[ ! "$NEW_MODULE" =~ ^[A-Za-z0-9._~/-]+$ ]]; then
    echo "错误: module 路径含非法字符: $NEW_MODULE"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TEMPLATE_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

if ! grep -q "^module ${OLD_MODULE}$" "$TEMPLATE_ROOT/go.mod" 2>/dev/null; then
    echo "错误: $TEMPLATE_ROOT/go.mod 不是模板工程（module 应为 ${OLD_MODULE}）"
    exit 1
fi

DEST_PARENT="${3:-$(dirname "$TEMPLATE_ROOT")}"
DEST_PARENT="$(cd "$DEST_PARENT" && pwd)"
DEST_DIR="$DEST_PARENT/$NEW_NAME"

if [ -e "$DEST_DIR" ]; then
    echo "错误: 目标目录已存在: $DEST_DIR"
    exit 1
fi

echo "========================================"
echo " 从模板创建新工程: $NEW_NAME"
echo " module:   $NEW_MODULE"
echo " 模板路径: $TEMPLATE_ROOT"
echo " 输出路径: $DEST_DIR"
echo "========================================"
echo ""

echo "[1/5] 复制模板..."
mkdir -p "$DEST_DIR"
# 用 tar 复制以保留符号链接；排除 .git、构建产物与本地配置
(cd "$TEMPLATE_ROOT" && tar --exclude=.git --exclude=bin --exclude='.claude/settings.local.json' --exclude=.DS_Store -cf - .) \
    | (cd "$DEST_DIR" && tar -xf -)
echo "  - 已复制到 $DEST_DIR"

cd "$DEST_DIR"

echo "[2/5] 替换 module 路径..."
ESC_OLD_MODULE="$(printf '%s' "$OLD_MODULE" | sed 's/[.[\*^$/]/\\&/g')"
ESC_NEW_MODULE="$(printf '%s' "$NEW_MODULE" | sed 's/[&/\]/\\&/g')"
CHANGED=()
while IFS= read -r f; do
    if grep -q "$OLD_MODULE" "$f"; then
        sedi "s/${ESC_OLD_MODULE}/${ESC_NEW_MODULE}/g" "$f"
        CHANGED+=("$f")
    fi
done < <(find . -type f \( -name '*.go' -o -name 'go.mod' -o -name '*.md' -o -name '*.sh' -o -name '*.json' -o -name 'Makefile' \) \
    -not -path './.git/*' -not -path './.ai/skills/init-project/*')
echo "  - 已替换 ${#CHANGED[@]} 个文件中的 module 路径"

echo "[3/5] 替换模板名称..."
ESC_OLD_NAME="$(printf '%s' "$OLD_NAME" | sed 's/[.[\*^$/]/\\&/g')"
ESC_NEW_NAME="$(printf '%s' "$NEW_NAME" | sed 's/[&/\]/\\&/g')"
while IFS= read -r f; do
    if grep -q "$OLD_NAME" "$f"; then
        sedi "s/${ESC_OLD_NAME}/${ESC_NEW_NAME}/g" "$f"
        CHANGED+=("$f")
    fi
done < <(find . -type f \( -name '*.md' -o -name '*.json' -o -name 'Makefile' -o -name 'smoke.sh' \) \
    -not -path './.git/*' -not -path './.ai/skills/*')
if [ -f "api/postman/${OLD_NAME}.postman_collection.json" ]; then
    mv "api/postman/${OLD_NAME}.postman_collection.json" "api/postman/${NEW_NAME}.postman_collection.json"
    echo "  - Postman 集合已重命名"
fi

echo "[4/5] 检查符号链接..."
relink() {
    local link="$1" target="$2"
    if [ ! -L "$link" ] || [ ! -e "$link" ]; then
        rm -rf "$link"
        mkdir -p "$(dirname "$link")"
        ln -s "$target" "$link"
        echo "  - 重建 $link -> $target"
    fi
}
relink CLAUDE.md .ai/ai-rules.md
relink AGENTS.md .ai/ai-rules.md
relink .claude/skills ../.ai/skills
relink .agents/skills ../.ai/skills

echo "[5/5] 验证构建与测试..."
if ! command -v go >/dev/null 2>&1; then
    echo "  - 警告: 未找到 go 命令，请手动在 $DEST_DIR 执行 make check"
else
    go build ./... && go test ./... >/dev/null
    echo "  - go build / go test 通过"
fi

echo ""
echo "========================================"
echo " 创建完成"
echo " 工程路径: $DEST_DIR"
echo "========================================"
echo ""
echo "修改的文件:"
printf '  %s\n' "${CHANGED[@]}" | sort -u
