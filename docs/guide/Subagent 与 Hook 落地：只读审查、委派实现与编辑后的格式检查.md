---
title: Subagent 与 Hook 落地：只读审查、委派实现与编辑后的格式检查
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - subagent
  - hook
  - spec-reviewer
  - spec-implementer
  - post-tool-use
---

# Subagent 与 Hook 落地：只读审查、委派实现与编辑后的格式检查

go-backend-template 里的 Subagent 和 Hook 只属于 Claude Code，两者都不在门禁链上。`.claude/agents/` 下放着两个定义文件：`spec-reviewer` 做只读审查，`spec-implementer` 在划定路径内做测试先行实现。`.claude/settings.json` 里配了一个 `PostToolUse` Hook，每次 `Edit` / `Write` 之后跑 `scripts/check-format.sh` 提醒 gofmt。要解决的是两件事：主会话需要一个没参与过实现的第二视角，最常见的低级偏差要尽早报出来。证据来自 `make check` 这一类命令，决定来自用户。

harness 的分工里，Skill 承载流程知识（见「Skill 落地」），Subagent 给独立视角和收窄的工具集，Hook 给事后反馈。这一篇只写后两者。

## 两个定义文件只靠三个字段划权限

Subagent 定义文件就是一段 Markdown。frontmatter 三个字段 `name`、`description`、`tools`，正文写角色说明。`.claude/agents/spec-reviewer.md` 的头部：

```yaml
---
name: spec-reviewer
description: Read-only review of a go-backend-template version for requirement coverage, implementation correctness, and production risk. Use after Step 6.1 self-verification passes and before user acceptance or document sync. Never edits files or runs commands.
tools: Read, Grep, Glob
---
```

`tools` 是这份文件里唯一的硬约束，两个角色的差别全压在这一行：

| 定义文件 | `tools` | 什么时候用 | 正文里的边界 |
|---|---|---|---|
| `.claude/agents/spec-reviewer.md` | `Read, Grep, Glob` | Step 6.1 自验通过之后、用户验收或文档同步之前 | 不修改文件，不运行命令，不把仓库里没有证据的检查写成通过 |
| `.claude/agents/spec-implementer.md` | `Read, Grep, Glob, Edit, Write, Bash` | 可选的 `spec-graph-workflow` 委派实现时 | 只在主会话给出的路径内工作；`Specs/requirements/**`、`Specs/technical/{version}/graph.json`、`.ai/skills/go-development/**` 始终禁止修改；不做最终验收结论 |

Reviewer 手里没有 `Edit`、`Write` 和 `Bash`。「不修改文件、不运行命令」不只写在正文里，工具集本身就是这个形状。Implementer 有 `Bash`，能跑 `go test`，路径禁令就只剩正文约束，加上主会话核对它返回的改动清单。`.ai/ai-rules.md` 的「文件权限规则」再补一条：新增预授权命令、Hook 或 Subagent 要在变更说明里告知用户。

有一点要说明白。两个文件放在 Claude Code 约定的 `.claude/agents/` 目录里，但原生加载和运行时工具限制在本模板里还没有端到端记录。1.0.0 的实现委派是用通用 Agent 加载定义文件正文完成的，下文写清楚。

## spec-reviewer 先读规则，再读代码，最后只交一份 YAML

读取顺序写死在 `spec-reviewer.md` 正文里。`.ai/ai-rules.md` 打头，接着 `Specs/requirements/{version}/需求.md` 与 `Specs/requirements/协议与数据.md`，然后是 `Specs/technical/{version}/技术方案.md`，包括其中的「测试计划」与「交付状态」，再到 `Specs/technical/技术讲解.md`。代码、测试、迁移、脚本与 Postman 集合排在最后，范围由技术方案「文件清单」给定。审查者的参照系被钉在需求和方案上，不从代码反推「应该是什么」。

随后是三轮审查，每轮盯一类问题：

| 轮次 | 检查什么 |
|---|---|
| 需求覆盖 | 每个 `F-{version}-NNN` 与其 AC-n 是否都有实现、对应测试用例名与验证方式；技术方案「需求摘要」标为「部分 / 否」的项是否有用户答复记录 |
| 实现正确性 | API 契约（状态码、`error.code`、字段）与代码一致；错误包装与映射；并发与资源退出路径；日志不泄露敏感信息；依赖方向 `cmd -> httpapi -> domain <- storage`；无无调用方的抽象 |
| 生产风险 | 迁移与回滚、配置默认值、超时与优雅停机、集成测试是否用真实依赖、smoke 与 Postman 集合是否与契约同步、技术讲解是否落后于代码 |

审查结束只交一份 YAML，文件里叫它 Handoff contract。`verdict` 取 `pass` / `changes_required` / `blocked`。`findings` 每条带稳定编号 `id`，从 R1 往下排；`severity` 取 P0 到 P3；`feature` 填 Feature 或 AC 编号，与需求无关的写 `general`；`location` 写成 `path/to/file.go:LINE`；剩下三项是 `failure_mode`、`minimum_fix`、`required_verification`。`verification_gaps` 列仓库中没有证据表明已执行的检查。`blocked` 只用于缺少必要输入，`pass` 也要列 `verification_gaps`。

合同末句划责任：finding 的接受、拒绝、修复与复验归主会话，涉及需求取舍的 finding 只能由用户决定。另一半在 `.ai/ai-rules.md` Step 6.1：审查结论不替代命令结果；做了审查就把技术方案「交付状态」的 `review` 写成 `pending` / `changes_required` / `pass`，没做保持 `not_required`。审查是第二视角，不是第二套门禁。

## spec-implementer 的边界是路径清单和 focused 测试

`spec-implementer.md` 的输入由主会话给出：版本号，技术方案里要实现的条目，也就是 Feature ID、AC 编号和文件清单子集，再加允许修改的路径列表。工作方式五条：先读 `.ai/ai-rules.md` 的「代码规范」与「测试」两节，再读方案条目；测试先行，先跑一次确认失败原因正确再写最小实现；只跑 focused 测试 `go test ./internal/<pkg>/...`，涉及集成加 `-tags integration`，不跑 `make check`、`make smoke`、`api-verify`；用 `gofmt -l` 检查自己改过的文件；技术讲解、技术方案「变更记录」、Postman 集合与 README 都不碰，留给主会话在 Step 7 统一回写。

返回内容也是固定 YAML：`scope`、`changed_files` 每条带 `path` 与 `kind: added | modified`、`tests` 每条带 `name`、`red_observed`、`green_command`，最后是 `open_questions`。

这样切分对应 loop engineering 的一条原则：Writer 只交局部 RED/GREEN 证据，broad 门禁在最终候选上由一个执行者跑一次。`spec-graph-workflow` 把这个执行者定为主会话，也就是唯一 Controller。Implementer 的 focused GREEN 不绑定候选身份，`spec-graph record` 登记的只能是 Controller 刚运行的命令及其退出码（见「Spec + Graph 落地」）。

## 1.0.0 的委派真实发生了什么

1.0.0 有两个功能，`F-1.0.0-001` 创建笔记 `POST /v1/notes` 与 `F-1.0.0-002` 读取笔记 `GET /v1/notes/{id}`，实现由 `spec-implementer` 角色完成。做法是用通用 Agent 加载 `.claude/agents/spec-implementer.md` 的正文当角色说明，不是 Claude Code 原生的 `agents` 目录加载。它只跑 focused 测试，返回 11 个改动文件和每个用例的 RED/GREEN 记录。`make check`、`make test-integration`、`make test-race`、`make smoke` 与 `api-verify` 由主会话作为唯一 Controller 运行，再用 `spec-graph record` 登记。

`spec-reviewer` 第一轮返回 `changes_required`，五条 finding：

| id | severity | 内容 | 处理 |
|---|---|---|---|
| R1 | P2 | `memstore.Store.Save` 在 ID 冲突时静默覆盖 | 改为返回 `note.ErrAlreadyExists`，不覆盖；先补 `TestStore_Save_DuplicateID`、`TestService_Create_SaveAlreadyExists` |
| R2 | P2 | `decodeJSON` 不检查尾随数据 | 二次 `Decode` 检查 `io.EOF`；`TestCreateNote_BadBody` 新增 `trailing json value`、`trailing garbage` 子用例 |
| R3 | P3 | 技术方案风险条目对 `crypto/rand` 失败的描述与 Go 1.24+ 行为不符 | Controller 回写技术方案 |
| R4 | P3 | 模块设计缺 `logger` 字段、测试计划未回写用例名 | Controller 回写技术方案 |
| R5 | P2 | 技术讲解与 README 落后于代码 | Controller 回写文档 |

R1 最值得看。需求写着「并发创建互不影响，不得丢失或覆盖」，返回错误比覆盖更严格地满足这句话，没有放宽需求，不属于「涉及需求取舍」，主会话可以直接接受。修复交回 `spec-implementer` 角色，测试先行，`internal/memstore/store.go` 最终是：

```go
func (s *Store) Save(_ context.Context, n note.Note) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.notes[n.ID]; exists {
		return note.ErrAlreadyExists
	}
	s.notes[n.ID] = n
	return nil
}
```

`note.Service.Create` 用 `save note: %w` 包装上抛，`httpapi` 把它落到 `writeServiceError` 的默认分支，500 `internal_error`，并记录日志。R2 的修复在 `internal/httpapi/notes.go` 的 `decodeJSON` 末尾：

```go
if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
	return errors.New("request body must contain a single JSON value")
}
```

R3、R4、R5 是文档问题，Controller 直接回写。第二轮审查返回 `pass`，新增 R6 P3：测试计划里一个用例名挂错，Controller 改为 `TestService_Create_SaveAlreadyExists`。六条 finding 最终全部 `verified`。

顺序留在 `Specs/technical/1.0.0/graph.json` 里：第一轮 `review` 记退出码 1，revision 8；R1 到 R5 登记为 `open`，9 到 13；`review_failed` 进入 `fixing`，14；逐条 `accepted` 15 到 19、`fixed` 20 到 24；重跑 `check` 后 `fix_done` 回到 `reviewing`，26；第二轮 `review` 退出码 0，40；`review_passed` 进入 `verifying`，41。finding 的状态只能沿 `open -> accepted | rejected`、`accepted -> fixed`、`fixed -> verified` 走，这张固定表在 `internal/specgraph/transition.go` 里。

审查 `pass` 之后版本并没有交付。graph 停在 `verifying`、revision 46，技术方案「交付状态」是 `stage: verifying`、`user_acceptance: pending`、`review: pass`。`event 1.0.0 verified` 被守卫以退出码 3 拒绝，理由是 `user_acceptance` 仍为 `pending`，需要 `confirmed`。Reviewer 能关掉 finding，关不掉用户验收（见「验证闭环」）。

## PostToolUse Hook 只是编辑后的一次 gofmt 提醒

Hook 的全部配置在 `.claude/settings.json` 的 `hooks` 一节：

```json
"PostToolUse": [
  {
    "matcher": "Edit|Write",
    "hooks": [
      {
        "type": "command",
        "command": "\"$CLAUDE_PROJECT_DIR\"/scripts/check-format.sh"
      }
    ]
  }
]
```

同一个文件的 `permissions.allow` 预授权了八条命令前缀：`go`、`gofmt`、`make`、`curl`、`git`、`bash`、`lsof`、`shasum`。`git` 用于 `init-project` / `sync-ai-assets` 的仓库初始化与克隆。`shasum` 有两个用处，复核 `.ai/skills/go-development/SOURCE.md` 的上游文件 SHA-256，以及 `docs/spec-graph/落地.md` 里剔除交付状态块后的技术方案摘要核对。其余六条门禁、冒烟和 `api-verify` 都会用到。

被调用的 `scripts/check-format.sh` 很短。它先 `cd` 到脚本目录的上一级也就是仓库根，用 `find cmd internal -type f -name '*.go'` 收集文件，没有 Go 文件直接 `exit 0`；有文件就跑 `gofmt -l`，列表非空时向 stderr 输出 `gofmt needed:` 和文件路径，`exit 2`。脚本头部注释把边界写死了：只是即时反馈，不替代 `make check`，也不覆盖通过 Bash 产生的改动。

脚本本身做过探针验证。放进一个未格式化的 `internal/hookprobe/unformatted.go`，从仓库外执行脚本，输出 `gofmt needed:` 与该路径，退出码 2；删除探针后退出码 0。Claude Code 会话里 `PostToolUse` 自动触发这一段还没有真实记录。

| 边界 | 含义 |
|---|---|
| 事后反馈 | Hook 在写入完成之后才运行，不能撤销已经落盘的内容 |
| 不覆盖 Bash 改动 | 通过 `sed`、heredoc 或脚本写出的文件不经过 `Edit` / `Write`，不会触发 |
| 不替代 `make check` | Makefile 的 `check` 目标用 `gofmt -l .` 检查整个仓库，Hook 只看 `cmd/` 与 `internal/` |
| 不是安全边界 | `.ai/ai-rules.md` 明确：真正需要禁止的操作依靠权限、沙箱和人工确认 |

## Codex 没有这两种机制，怎么办

`.codex/config.toml` 里只有 `personality = "friendly"` 和一段注释，没有 `mcp_servers`；仓库也没有 `.mcp.json`（见「MCP 落地」）。注释和 `.ai/ai-rules.md` 的「Subagent 与 Hook」一节给出同一套替代：格式检查靠 `make check`；独立审查在新会话中按 `.claude/agents/spec-reviewer.md` 正文执行，返回同一份 Handoff YAML。`spec-graph-workflow` 也写明 Codex 可以只用 `spec-graph` CLI 记录状态，实现与审查在同一会话里串行完成。

审查者是不是一个独立进程，合同不变：先读规则再读代码，只返回 verdict 与 findings，finding 归主会话处理。少掉的是工具集收窄和会话隔离。在 Codex 里要多核对一件事：审查过程有没有顺手改文件。

## 什么时候委派，什么时候留在主会话

| 场景 | 做法 | 依据 |
|---|---|---|
| Step 6.1 自验通过，需要独立视角 | 委派 `spec-reviewer` | `.ai/ai-rules.md` Step 6.1「可选独立审查」 |
| 版本较大、切片互不重叠 | 用 `spec-graph-workflow` 委派一个或多个 `spec-implementer` | Skill 的 Step B |
| 日常小修改（bug 修复、配置调整） | 留在主会话，按「日常代码修改」五步走 | `.ai/ai-rules.md` |
| finding 涉及需求取舍 | 停下来问用户，不委派 | Handoff contract 末句 |
| 运行 `make check` 等 broad 门禁、`spec-graph record`、修改「交付状态」、Step 7 回写 | 只能主会话 | `spec-implementer.md`、Skill「角色」表 |

判断标准压缩成一句：需要收窄工具集或隔离上下文才委派，需要决定、需要证据身份或需要与用户对话就留在主会话。Subagent 的返回值和 Hook 的退出码都是输入。结论由 `make check` 这一类命令的退出码，以及技术方案「交付状态」里用户写下的 `confirmed` 决定。
