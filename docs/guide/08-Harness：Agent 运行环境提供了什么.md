---
title: Harness：Agent 运行环境提供了什么
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - concept
  - spec-coding
---

# Harness：Agent 运行环境提供了什么

## 概念

harness 指承载编码 Agent 的那层运行环境。同一个模型、同一份提示词，放在不同 harness 里行为并不一样，差别来自这层环境提供的五件事。

- **规则注入**：会话开始时自动读哪些文件。Claude Code 读 `CLAUDE.md`，Codex 读 `AGENTS.md`。
- **工具集**：能不能编辑文件、能不能跑 shell、有没有外接的 MCP Server。
- **权限判定**：哪些命令直接执行，哪些要弹确认，哪些禁止。
- **生命周期事件**：能不能在「工具调用之后」这类节点挂自己的检查脚本。
- **子会话**：能不能开一个上下文独立、工具集受限的 Subagent。

Agent 的行为一半来自提示词，一半来自这层环境。提示词说「改完记得格式化」是提醒，Hook 在每次写文件后跑 gofmt 是机制；提示词说「不要改需求文档」是约定，权限配置拒绝写入才是边界。

麻烦的地方在于这层环境换个 Agent 就不通用。Hook 是 Claude Code 的机制，Codex 没有；MCP Server 装没装取决于用户机器。模板不能假设某个特性一直在，否则换一个 harness 就变成一堆跑不起来的说明。

## 本工程怎么落地

### 规则入口只有一份

`CLAUDE.md` 和 `AGENTS.md` 都是指向 `.ai/ai-rules.md` 的符号链接，两个 Agent 会话一开始读到的是同一份文本。

`.claude/skills` 与 `.agents/skills` 同样是指向 `../.ai/skills` 的符号链接，`api-verify`、`init-project`、`spec-coding-init`、`spec-graph-workflow`、`sync-ai-assets`、`go-development` 六个 Skill 目录只存一份。

这四条链接不靠约定俗成维持。`internal/speccheck/speccheck.go` 里的 `requiredSymlinks` 把它们写成了必查项：

```go
var requiredSymlinks = map[string]string{
	"CLAUDE.md":      ".ai/ai-rules.md",
	"AGENTS.md":      ".ai/ai-rules.md",
	".claude/skills": "../.ai/skills",
	".agents/skills": "../.ai/skills",
}
```

`checkSymlinks` 既验证目标路径，也确认它确实是符号链接而不是复制出来的普通文件。`make check` 里的 `spec-check` 跑的就是这个，链接断了、被人复制成两份文件，检查会失败。

### 工具授权写在 settings.json

`.claude/settings.json` 的 `permissions.allow` 列出八条命令前缀：`Bash(go:*)`、`Bash(gofmt:*)`、`Bash(make:*)`、`Bash(curl:*)`、`Bash(git:*)`、`Bash(bash:*)`、`Bash(lsof:*)`、`Bash(shasum:*)`。

范围正好覆盖 `make check`、`make smoke`、`api-verify` 需要的动作，跑门禁时不必反复确认。列表外的命令仍然走确认流程。

### 事件钩子挂在 PostToolUse

同一份 settings 里，`hooks.PostToolUse` 用 `"matcher": "Edit|Write"` 匹配写文件类工具，之后调用 `"$CLAUDE_PROJECT_DIR"/scripts/check-format.sh`。

脚本只做一件事：`find cmd internal -type f -name '*.go'` 收集文件，`gofmt -l` 检查，有未格式化的就打到 stderr 并 `exit 2`。

它的边界写在脚本注释里：事后反馈，不替代 `make check`，也不覆盖通过 Bash 产生的改动。Hook 在写入发生之后才运行，撤不回已经落盘的内容；Agent 用 `sed` 改文件时它根本不触发。

所以 `.ai/ai-rules.md` 的 Step 5 把它定位成「gofmt 即时提醒」，真正的格式门禁留在 `make check` 的 `gofmt -l .` 那一行。

### 子会话有两个角色，各自限定 tools

`.claude/agents/spec-reviewer.md` 的 frontmatter 写 `tools: Read, Grep, Glob`，正文第一句是不修改任何文件、不运行任何命令、不把仓库里没有证据的检查写成通过。只读是配置层保证的，不靠提示词自律。它按需求覆盖、实现正确性、生产风险三轮审查，返回带 `verdict` 和 `findings` 的 YAML 交接合同。

`.claude/agents/spec-implementer.md` 的 tools 多了 `Edit, Write, Bash`，但范围被正文卡死：`Specs/requirements/**`、`graph.json`、`.ai/skills/go-development/**` 始终禁止修改；只跑 focused 测试，`make check`、`make smoke`、`api-verify` 由主会话作为唯一执行者运行并记录证据。

### 没有 MCP 依赖

`.codex/config.toml` 里没有 `mcp_servers` 段，实际配置只有一行 `personality = "friendly"`；仓库根目录也没有 `.mcp.json`。

文件开头的注释说明了取向：构建与测试走本地 Go 工具链，接口验证走 `make smoke` / curl / Postman 集合。

`.ai/ai-rules.md` 的「能力前置校验」表里，Postman MCP 明确标成可选，缺失时改用手动导入集合或 curl，末尾一句是「本工程的必需门禁不依赖任何 MCP Server」。

### 环境差异如实标注

`.ai/ai-rules.md` 末尾的「Subagent 与 Hook（Claude Code）」表把三个机制的位置、作用、边界逐行列出，收尾写明 Codex 无对应机制：审查在新会话中按 `spec-reviewer` 正文执行，格式检查依赖 `make check`。Step 6.1 的独立审查段落也写明 Codex 没有 Subagent 机制时在新会话中按该文件正文执行同一审查。

`.codex/config.toml` 开头的注释重复了同一组替代动作：格式检查依赖 `make check`，独立审查按 `spec-reviewer` 正文在新会话中执行。

## 扩展思路

### 换 harness 或加 Agent 时，先列这四样

规则入口读哪个文件、工具怎么授权、事件钩子挂在哪、子会话有哪些角色。四样里缺哪一样，就明确写出缺失时用什么替代。

这套模板的答案是：规则入口靠符号链接归一，工具授权靠 `permissions.allow`，钩子靠 `PostToolUse`，子会话是 `spec-reviewer` 与 `spec-implementer` 两个角色。Codex 缺后两样，替代方案是 `make check` 加新会话执行同一份审查说明。

### 能力有无用前置校验表声明

`.ai/ai-rules.md` 的「能力前置校验」给每项能力配了探针和缺失动作：Go 工具链探针是 `go version` 输出版本不低于 go.mod 声明，缺失时 STOP；make 探针是 `make -v`；Skill 的探针是系统提示中的可用 Skill 列表，不在列表里就视为不可用。

必需能力缺失必须 STOP 并说清缺什么怎么装，不得绕过或用其他方式凑合。可选能力（git、Postman MCP、golangci-lint、govulncheck）缺失只需说明改用什么。

给新增能力扩这张表，比在正文里塞一段「如果没装 X 就……」更容易被 Agent 真正执行。

### 禁止交给权限和沙箱，Hook 和规则只做提醒

`.ai/ai-rules.md` 的收尾句是真正需要禁止的操作依靠权限、沙箱和人工确认。Hook 跑在写入之后，规则文本可以被绕过，两者都不是安全边界。

要防「Agent 改了不该改的文件」，靠权限规则和人工 review；要防「提交了没格式化的代码」，靠 CI 里的 `make check`。把 Hook 当围栏用，早晚会在某次 Bash 改动上漏掉。

### 给 harness 特性留降级路径

判断标准很简单：把 Hook、Subagent、MCP 全部拿掉，这套流程还能不能跑通。

这里能。`make check`（`go vet` + `go test ./...` + `gofmt -l .` + `spec-check`）、`make test-integration`、`make test-race`、`make smoke` 全是本地命令，`spec-check` 也是 `go run ./cmd/spec-check` 而不是某个插件。Subagent 只是把审查搬进独立上下文，审查标准写在 markdown 正文里，人照着做一遍结果一样。

新增一个依赖 harness 特性的能力时，先回答降级问题：这个特性不在的时候，同样的检查用什么命令完成？答不上来，这个能力就还不能进模板。
