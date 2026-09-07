---
title: Rules 落地：ai-rules.md 单源怎样被两个 Agent 共读并被机器守住
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - rules
  - ai-rules
  - claude-code
  - codex
---

# Rules 落地：ai-rules.md 单源怎样被两个 Agent 共读并被机器守住

Claude Code 开新会话先读 `CLAUDE.md`，Codex 先读 `AGENTS.md`。两份文件各写一遍规则，用不了多久就会分叉：一份要求跑 `make test-integration`，另一份还停在旧命令上。go-backend-template 里只有一份源文件 `.ai/ai-rules.md`，两个入口都是指向它的符号链接。`make check` 里的 `spec-check` 再核对这份文件的关键章节和链接本身，链接被删、被换成普通文件或指错目标时以退出码 1 拒绝。

## 为什么规则只有一份源文件

共读关系由四个符号链接决定，两个在根目录，两个在 `.claude/` 与 `.agents/` 下：

| 链接 | 目标 | 谁读 |
|---|---|---|
| `CLAUDE.md` | `.ai/ai-rules.md` | Claude Code 会话入口 |
| `AGENTS.md` | `.ai/ai-rules.md` | Codex 会话入口 |
| `.claude/skills` | `../.ai/skills` | Claude Code 的 `/{skill}` |
| `.agents/skills` | `../.ai/skills` | Codex 的 `${skill}` |

`.ai/` 目录只放两样东西：`ai-rules.md` 和 `skills/`。Claude Code 特有的 `.claude/settings.json` 与 `.claude/agents/`、Codex 特有的 `.codex/config.toml` 留在各自目录，不进单源。

建好的链接会坏，两处流程负责修。`.ai/skills/init-project/rename_module.sh` 的 `[1/5]` 用 `tar` 复制模板，排除 `.git`、`bin`、`.claude/settings.local.json`、`.DS_Store`，tar 保留符号链接；`[2/5]` 替换 module 路径时排除 `.ai/skills/init-project`，`[3/5]` 替换模板名时排除整个 `.ai/skills`；`[4/5]` 对四个链接逐一调用 `relink`，缺失、断裂或已变成普通文件时删掉重建：

```bash
# .ai/skills/init-project/rename_module.sh
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
```

Skill `sync-ai-assets` 在 2.5 检查同样四个链接，断裂或缺失记为「需修复」，Step 5 重建。合并 `ai-rules.md` 按二级标题拆分：模板新增的章节插入，本项目自有章节保留，「项目简介」「目录结构」「当前公共实现清单」默认保留本地内容，只做提示。

还有一条不属于门禁的事实。`Specs`、`.ai`、`.claude`、`.codex`、`.agents`、`api` 等 16 个工作流目录在 macOS 上带红色标签。标签存在扩展属性里，`rename_module.sh` 的 tar 复制会保留，`git clone` 不会携带。它只是视觉区分。

## ai-rules.md 每一节在回答什么

| 章节 | 回答的问题 | 落到哪里 |
|---|---|---|
| 项目简介 | 这是什么服务，语言、依赖、架构、必需验证是什么 | `init-project` Step 3 把第一段改成业务描述 |
| 目录结构 | 每个目录归谁维护 | 与「文件权限规则」互为索引 |
| 文件权限规则 | 哪些路径 AI 禁改、维护、可改但要告知 | `spec-implementer` 禁止路径，Step 2 提问 |
| 能力前置校验 | 执行前探针什么，缺失时 STOP 还是降级 | `api-verify` 前置校验，`make lint` / `make vuln` 自动跳过，见「MCP 落地」 |
| AI 工作流（版本开发） | Step 1 到 Step 7 的输入、输出与停止条件 | 见「版本开发工作流」 |
| 日常代码修改（非版本开发） | 不走版本流程的改动必须做的五件事 | 见「代码规范与日常修改」 |
| 代码规范 | 包组织、命名、错误、HTTP、并发、日志、测试、DRY、依赖 | `spec-reviewer` 读取顺序第 1 项 |
| 验证命令 | 每条命令的内容与何时必跑 | `Makefile` 目标，见「Command 落地」 |
| Skills | 六个 Skill 的用途与调用方式 | `.ai/skills/`，见「Skill 落地」与「第三方 Skill」 |
| Subagent 与 Hook（Claude Code） | 两个 Subagent 与一条 Hook 的位置、作用、边界 | `.claude/agents/`、`.claude/settings.json`，见「Subagent 与 Hook 落地」 |

### 文件权限规则是逐路径写的

| 路径 | 规则 |
|---|---|
| `Specs/requirements/**` | AI 禁止修改；歧义时提问，不替用户改需求 |
| `Specs/technical/**` | AI 维护，必须反映代码最新状态 |
| `api/postman/**`、`scripts/**` | AI 维护，接口变更时同步 |
| `cmd/**`、`internal/**`、`go.mod`、`go.sum`、`Makefile` | AI 可新增和修改；改 Makefile 目标或依赖时同步 README 与技术讲解 |
| `README.md` | 只在路由、配置、命令、Skills 变化时同步，不放进度 |
| `.claude/settings.json`、`.claude/agents/**`、`.codex/config.toml`、`.gitignore` | 可修改，但新增预授权命令、Hook、Subagent 或忽略项要告知用户 |
| `Specs/technical/{version}/graph.json` | 只能通过 `spec-graph` 命令写入；记录执行阶段与证据身份，不记录用户决定 |
| `.ai/ai-rules.md` | 可演进，但不得删除「文件权限规则」「AI 工作流」「验证」三节 |
| `.ai/skills/{自研 Skill}/**` | 可按真实需要修改，改动在 Skill 内保持步骤自洽 |
| `.ai/skills/{第三方 Skill}/**`（含 `SOURCE.md`） | AI 禁止修改，只能整目录升级并更新 `SOURCE.md` |

1.0.0 演示版本里三条规则都留下了痕迹。`Specs/requirements/1.0.0/需求.md` 由 Agent 在用户授权下代写，落盘后按人工需求只读；`graph.json` 从 revision 0 推进到 46，每一次写入都经 `go run ./cmd/spec-graph`；第三方 Skill `go-development` 1.15.1 的 26 个上游文件全部在 `SOURCE.md` 记录 SHA-256，本地改动为零。

### 能力前置校验把缺工具分成 STOP 和降级两类

表里七行能力分两类。Go 工具链、make、curl、Skill 四行必需，缺失必须 STOP，并说明缺什么、怎么装；git、Postman MCP、golangci-lint / govulncheck 三行标「可选」，缺失只需说明替代方式。必需门禁不依赖任何 MCP Server，完整探针表与分级见「MCP 落地」。门禁记录两侧都有例子：本机 Go 1.27.0 满足声明，四条必需命令全部通过；`make lint` 与 `make vuln` 因工具未安装按设计跳过；没有 Postman MCP，`api-verify` 用 curl 跑完 17 项场景。

### 工作流、代码规范与验证命令在这里只点名

「AI 工作流（版本开发）」开头有一条「需求变更与新增规则」：无论变更大小，都要重新读需求找变更点，更新技术方案相关章节，向用户展示并确认后才编码，编码后照常执行 Step 6 与 Step 7。七个 Step 归「版本开发工作流」，这里只记它最硬的停止条件，写在 6.2：用户最终确认前 Step 6 不算完成，不得进入 Step 7。

「代码规范」有九个小节。最容易被忽略的是「DRY 红线」末尾的公共实现清单，只有三行：JSON 响应用 `httpapi.writeJSON` / `httpapi.writeError`，请求日志与 panic 兜底用 `httpapi.withRequestLog` / `httpapi.withRecover`，环境变量用 `config.Load`。规则要求新增公共件后回填，`sync-ai-assets` 合并时默认保留本地版本。

「验证命令」表把 `make check`、`make test-integration`、`make test-race`、`make smoke` 与 Skill `api-verify` 标为必需，`make lint` / `make vuln` 与 Postman 标为可选，`make run` 归用户验证。

## 哪些章节被 spec-check 守住

`internal/speccheck/speccheck.go` 里有两个变量直接对应两条规则：

```go
// internal/speccheck/speccheck.go
var rulesHeadings = []string{"## 文件权限规则", "## AI 工作流（版本开发）", "## 验证命令"}

var requiredSymlinks = map[string]string{
	"CLAUDE.md":      ".ai/ai-rules.md",
	"AGENTS.md":      ".ai/ai-rules.md",
	".claude/skills": "../.ai/skills",
	".agents/skills": "../.ai/skills",
}
```

规则 2 由 `checkHeadings(rel, data, rulesHeadings, true)` 执行。三个二级标题各恰好出现一次，围栏代码块内的行不计，`### 文件权限规则` 这种三级标题不算，重复出现同样报问题。它守住的就是「文件权限规则」自己写的那条不得删除三节的约束。「项目简介」「代码规范」「Skills」等其余章节不在机器守护范围内，靠 `sync-ai-assets` 的合并策略和审查保留。

规则 4 由 `checkSymlinks` 执行：`os.Lstat` 确认是符号链接，`os.Readlink` 的结果必须与表中目标逐字相等，最后 `os.Stat` 确认目标存在。四种失败各有固定文案：「缺少符号链接，应指向 …」「不是符号链接，应指向 …」「符号链接目标应为 …，实际为 …」「符号链接目标不存在: …」。把 `CLAUDE.md` 换成一份复制出来的普通文件，内容一字不差，`spec-check` 照样以退出码 1 拒绝。

规则 1 还要求 `.ai/ai-rules.md` 存在且是普通文件。三条规则都跑在 `make check` 里（`check: vet test spec-check`），仓库输出 `spec-check ok (1 version(s))`。

## 规则怎样落到动作

| 规则 | 对应的 Step、命令或文件 |
|---|---|
| 人工需求只读 | Step 2 提问；`spec-implementer` 的禁止路径；`spec-check` 规则 6 只从需求读取 Feature ID 做追溯 |
| 技术方案确认后才编码 | Step 4 第 3 项；「交付状态」`stage` 改 `implementing`；`spec-graph event plan_confirmed` 守卫读取它 |
| 测试先行 | Step 5；`spec-implementer` 返回每个用例的 `red_observed` 与 `green_command` |
| 必需门禁 | Step 6.1 第 1 到 5 项：`make check`、`make test-integration`、`make test-race`、`make smoke`、`api-verify` |
| 用户确认才算交付 | 6.2 第 4 项 `user_acceptance: confirmed`；`specdoc` 的 delivered 门禁；`spec-check` 规则 8 |
| 编辑后格式检查 | `.claude/settings.json` 的 `PostToolUse` 运行 `scripts/check-format.sh`；`make check` 的 `gofmt -l` |
| 新增依赖先告知 | Step 2 的依赖告知条目与「依赖使用」；`go.mod` 当前只有标准库 |
| 文档同步 | Step 7 的七项回写；`spec-check` 规则 6、7 核对技术方案的 Feature ID 与八个章节 |

用户确认这条规则，在 1.0.0 上走到了机器拒绝那一步。六个 finding 全部 verified、最终门禁记录到 revision 46 之后，`go run ./cmd/spec-graph check 1.0.0` 退出码 0，`event 1.0.0 verified` 却被守卫挡住：「守卫不满足: verified: 技术方案「交付状态」user_acceptance 为 pending，需要 confirmed」，退出码 3。技术方案交付状态是 `stage: verifying`、`user_acceptance: pending`、`review: pass`。版本仍未交付。

## Codex 与 Claude Code 差在哪

| 项目 | Claude Code | Codex |
|---|---|---|
| 规则入口 | `CLAUDE.md` -> `.ai/ai-rules.md` | `AGENTS.md` -> `.ai/ai-rules.md` |
| Skills | `.claude/skills` -> `../.ai/skills`，调用 `/{skill}` | `.agents/skills` -> `../.ai/skills`，调用 `${skill}` |
| 预授权 | `settings.json` 的 8 条 `permissions.allow`：go、gofmt、make、curl、git、bash、lsof、shasum | 无对应项 |
| Hook | `PostToolUse` 在 Edit 与 Write 之后运行 `scripts/check-format.sh` | 无；格式检查依赖 `make check` |
| Subagent | `spec-reviewer`（tools 仅 `Read, Grep, Glob`）、`spec-implementer` | 无；审查在新会话按 `spec-reviewer.md` 正文执行 |
| MCP | 无 `.mcp.json` | `config.toml` 无 `mcp_servers`，只有 `personality` 与注释 |

`settings.json` 里 Hook 的 matcher 是 `Edit|Write`，命令是 `"$CLAUDE_PROJECT_DIR"/scripts/check-format.sh`。`.codex/config.toml` 的注释把差异写进了配置本身：本工程不依赖任何 MCP Server，Hook 与 Subagent 在 Codex 中没有对应机制。`spec-graph-workflow` 对 Codex 的说明一致，可以只用 `spec-graph` CLI 记录状态，实现与审查在同一会话串行完成。

两条证据边界要如实写。`scripts/check-format.sh` 的脚本行为已用探针验证，真实 Claude Code 会话中的 `PostToolUse` 自动触发尚未记录。两个 Subagent 定义已配置，Claude Code 原生加载与运行时工具限制没有端到端记录，1.0.0 的实现是用通用 Agent 加载 `spec-implementer.md` 正文完成的，探针细节见「Subagent 与 Hook 落地」。`PostToolUse` 是事后反馈，撤销不了写入；Subagent 的结论不替代命令结果。

## 常见误区：把规则写成提醒，不写成可核对的动作

「注意保持文档同步」是提醒。可核对的版本是 Step 7 列出的七个回写对象，加上 `spec-check` 规则 6 核对每个 `F-{version}-NNN` 都出现在技术方案。「必须经过用户确认」同理，落到 `user_acceptance: confirmed` 这个字段、`specdoc` 门禁和 `spec-graph` 守卫上才有人执行。「不要改需求」对应 `spec-implementer` 的禁止路径，以及规则 10 拒绝版本目录里的意外文件。

边界也要承认。`ai-rules.md` 里还有一批规则只能靠人和审查核对：项目简介是否已改成业务描述，DRY 红线有没有被绕过，新增依赖有没有先告知。1.0.0 的 R1（`memstore.Store.Save` 遇到 ID 冲突静默覆盖）和 R2（`decodeJSON` 不检查尾随数据）是 `spec-reviewer` 第一轮审查找出来、再由测试先行修复的，`spec-check` 十条规则没有一条能发现它们。写规则时把三类分开标明：哪些由命令拒绝，哪些由审查发现，哪些必须由用户决定。
