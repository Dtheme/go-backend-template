---
name: sync-ai-assets
description: 从 go-backend-template 模板工程同步最新的共享 AI 资产到当前业务工程。支持 Skills、行为准则、Specs 模版、配置文件、Subagent 定义、脚本与工具包（spec-check、spec-graph）的增量对比与合并，展示差异后经用户确认才执行。
---

# /sync-ai-assets — AI 资产同步

> 用法：`/sync-ai-assets [模板工程路径]`（Codex：`$sync-ai-assets [模板工程路径]`）
> 模板源可以是本地目录，也可以是 git 地址（会 `git clone --depth 1` 到临时目录）。

## 交互规则

- 展示完变更摘要后，等待用户确认才执行
- 冲突项逐一展示差异并询问
- 用户输入 `跳过` 时跳过该项，继续下一个
- 本项目自有的 Skill、章节、配置一律保留，不删除

---

## 执行流程

### Step 1 — 定位模板

```bash
SRC="{参数}"
if [[ "$SRC" == *.git || "$SRC" == git@* || "$SRC" == http* ]]; then
    SYNC_TMP=$(mktemp -d) && git clone --depth 1 "$SRC" "$SYNC_TMP" && SRC="$SYNC_TMP"
fi
grep -q 'go-backend-template' "$SRC/go.mod" || { echo "不是 go-backend-template 模板"; exit 1; }
```

未传参数时询问用户模板路径。下文「模板」均指 `$SRC`。

### Step 2 — 逐类对比

#### 2.1 Skills

对比 `$SRC/.ai/skills/*` 与 `.ai/skills/*`，逐 Skill 目录内所有文件：

- 模板有、本项目无 → 「新增」
- 两端都有、内容不同 → 「更新」，展示 diff 摘要
- 内容相同 → 「无变化」
- 本项目有、模板无 → 「仅本地」，不动
- 第三方 Skill（目录内有 `SOURCE.md`，如 `go-development`）：以 `SOURCE.md` 中的版本号比较，模板版本更新时整目录替换，不做逐文件合并

#### 2.2 行为准则 `.ai/ai-rules.md`

按二级标题（`##`）拆分章节：

- 模板有、本项目无的章节 → 「新增」，插入对应位置
- 两端都有、内容不同 → 「更新」，展示关键差异；**项目简介、目录结构、当前公共实现清单**三处默认保留本项目内容，只提示
- 本项目自有章节 → 保留

#### 2.3 Specs 模版

| 模板 | 本项目 | 策略 |
| --- | --- | --- |
| `Specs/requirements/需求模版.md` | 同路径 | 直接覆盖（纯模版） |
| `Specs/technical/技术方案模版.md` | 同路径 | 直接覆盖（纯模版） |
| `Specs/requirements/协议与数据.md` | 同路径 | **不同步**（人工维护的业务内容） |

#### 2.4 配置与脚本

| 模板 | 本项目 | 策略 |
| --- | --- | --- |
| `.claude/settings.json` | 同路径 | 合并 `permissions.allow` 与 `hooks.PostToolUse`，保留本项目其他键 |
| `.claude/agents/*.md` | 同路径 | 不存在则复制；已存在且内容不同则展示差异，用户决定（`spec-reviewer` 的 `tools` 必须保持 `Read, Grep, Glob`） |
| `scripts/check-format.sh`、`scripts/spec-init.sh` | 同路径 | 不存在则复制；不同则「更新」（纯工具脚本，以模板为准） |
| `.codex/config.toml` | 同路径 | 展示差异，用户决定 |
| `.gitignore` | 同路径 | 确保包含模板条目，其余保留 |
| `Makefile` | 同路径 | 对比 `check`（必须依赖 `spec-check`）、`spec-check`、`spec-init`、`test-integration`、`test-race`、`smoke` 六个目标的定义（与 `spec-coding-init` 2.4 同一清单），缺失或定义不同则提示追加 / 更新 |
| `scripts/smoke.sh` | 同路径 | 展示差异，用户决定（本项目通常已按接口改写） |

#### 2.5 符号链接

检查 `CLAUDE.md`、`AGENTS.md` → `.ai/ai-rules.md`，`.claude/skills`、`.agents/skills` → `../.ai/skills`；断裂或缺失记为「需修复」。

#### 2.6 工具包与说明文档

这些目录是模板提供的工具代码，业务工程不应改动它们，以模板为准：

| 模板 | 策略 |
| --- | --- |
| `cmd/spec-check/`、`internal/speccheck/`、`internal/specdoc/` | `make check` 依赖，本项目缺失则「新增」；内容不同则「更新」并展示 diff 摘要 |
| `cmd/spec-graph/`、`internal/specgraph/`、`docs/spec-graph/`、`.ai/skills/spec-graph-workflow/` | 可选扩展：本项目已有则同上更新；本项目没有则询问是否启用，用户拒绝记为「已跳过」 |

工具包更新后必须在 Step 6 用 `make check` 确认编译与测试通过；本项目若已有 `Specs/technical/{version}/graph.json`，还要运行 `go run ./cmd/spec-graph check {version}`。

### Step 3 — 展示变更摘要

```
## AI 资产同步 — 变更摘要

模板: {SRC}

### Skills
- [无变化] init-project
- [更新]   api-verify — Step 2 新增幂等场景
- [新增]   new-skill
- [仅本地] project-custom-skill

### 行为准则
- [更新] 代码规范 / 测试 — 新增 -race 说明
- [保留] 项目简介、目录结构、当前公共实现清单

### Specs 模版
- [更新] 需求模版.md
- [无变化] 技术方案模版.md

### 配置与脚本
- [更新] .claude/settings.json — 新增 1 条 allow
- [无变化] .codex/config.toml、.gitignore、Makefile、.claude/agents/*.md、scripts/check-format.sh、scripts/spec-init.sh
- [待定] scripts/smoke.sh — 展示差异后由用户决定

### 符号链接
- 全部正常

### 工具包
- [更新] internal/speccheck — 新增规则 10（意外文件）
- [已跳过] cmd/spec-graph、internal/specgraph、docs/spec-graph — 用户未启用
```

### Step 4 — 用户确认

用户确认 → 按范围执行；拒绝或要求跳过某项 → 按指示执行。

### Step 5 — 执行同步

1. Skills：复制更新 / 新增的文件到 `.ai/skills/`
2. 行为准则：用 Edit 按章节合并
3. Specs 模版：直接复制覆盖
4. 配置、Subagent 定义、脚本：用 Edit 精确合并或复制
5. 符号链接：重建断裂或缺失项
6. 工具包与 docs：整目录复制覆盖（这些目录不含业务代码）

### Step 6 — 清理与验证

```bash
[ -n "${SYNC_TMP:-}" ] && rm -rf "$SYNC_TMP"
make check
```

`make check` 结果与同步前一致才算完成（同步不应影响构建）。

### Step 7 — 输出同步报告

```
## 同步完成

模板: {SRC}
已同步: X 项    已跳过: Y 项    无变化: Z 项

修改的文件:
- .ai/skills/api-verify/SKILL.md
- .ai/ai-rules.md
- Specs/requirements/需求模版.md
```
