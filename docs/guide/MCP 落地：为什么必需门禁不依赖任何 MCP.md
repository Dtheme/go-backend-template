---
title: MCP 落地：为什么必需门禁不依赖任何 MCP
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - mcp
  - postman
  - capability-check
---

# MCP 落地：为什么必需门禁不依赖任何 MCP

新会话接手 `go-backend-template`，第一反应往往是找工具：有没有数据库 MCP，有没有 Postman MCP，能不能让某个 Server 代跑接口。仓库里翻不到 `.mcp.json`，`.codex/config.toml` 里也没有 `mcp_servers`。

缺的不是配置。`make check`、`make test-integration`、`make test-race`、`make smoke` 和 Skill `api-verify` 这五道必需门禁，跑起来只用本地 Go 工具链和 curl。仓库里被提到的 MCP 只有 Postman MCP，它真正承担动作的位置有三个：能力前置校验表的可选行、`api-verify` 的附加步骤、Step 6.2 的用户验收选项。README、技术讲解、`.ai/ai-rules.md` 的项目简介与验证命令表，还有 `api-verify` 末节「与用户验证的关系」，提到它的地方都是这三处的引用。

MCP 是工具来源。证据只能来自在当前候选上真实跑过的命令，MCP 给不出这种东西，它在本模板里的位置就到此为止。

## 仓库里到底有没有 MCP 配置

`.codex/config.toml` 全文是一段注释加一个 `personality` 字段：

```toml
# .codex/config.toml
# Codex Agent 配置
#
# 本工程不依赖任何 MCP Server：构建与测试走本地 Go 工具链（make check，含 spec-check），
# 接口验证走 make smoke / curl / Postman 集合（api/postman/）。
# Skills 通过 .agents/skills -> ../.ai/skills 符号链接共享，行为准则见 AGENTS.md。
# Claude Code 的 PostToolUse Hook 与 .claude/agents 下的 Subagent 在 Codex 中没有对应机制：
# 格式检查依赖 make check，独立审查按 .claude/agents/spec-reviewer.md 正文在新会话中执行。

personality = "friendly"
```

`.claude/settings.json` 也只有两个键：`permissions.allow` 的 8 条预授权，go、gofmt、make、curl、git、bash、lsof、shasum，以及 `hooks.PostToolUse`。在模板自有文件里搜 `mcp`，命中落在 `README.md`、`Specs/technical/技术讲解.md`、`.codex/config.toml` 的注释、`.ai/ai-rules.md` 和 `.ai/skills/api-verify/SKILL.md`。每一处要么在说 Postman，要么在重复「不依赖任何 MCP Server」这句声明。

五道必需门禁实际调用的东西都能在 `Makefile`、`scripts/smoke.sh` 与 `.ai/skills/api-verify/SKILL.md` 里找到：

| 门禁 | 实际执行 | 结果 |
| --- | --- | --- |
| `make check` | `go vet ./...`、`go test ./...`、`gofmt -l .`、`go run ./cmd/spec-check` | 通过，`spec-check ok (1 version(s))` |
| `make test-integration` | `go test -tags integration ./...` | 通过 |
| `make test-race` | `go test -race ./...` | 通过 |
| `make smoke` | `bash scripts/smoke.sh`：编译真实二进制、启动、curl 9 项检查 | 通过 |
| Skill `api-verify` | `go build` 到 `mktemp -d` 目录、端口 18090 启动、逐接口 curl | 17 项场景通过 |

跑这些命令要装的东西就三样：Go 工具链、make、curl。能力前置校验表有四条必需行，其中三条正是这三项外部工具，第四条 Skill 不算外部工具。本机验证运行时是 Go 1.27.0 darwin/arm64，`go.mod` 声明 `go 1.25.0`，module 只用标准库。没有一步需要向 MCP Server 发请求。

## Postman MCP 在哪三个位置承担动作

第一个位置是 `.ai/ai-rules.md` 的能力前置校验表。这一行的探针写的是「系统提示中存在 `mcp__postman__*` 工具」，缺失时的处理是「不可用时不阻塞：用户验收改用手动导入集合或 curl」。探针看的是工具列表，不看配置文件，模板不用为它准备任何 MCP 配置。

第二个位置在 `api-verify`。前置校验第 3 条写明：有 `mcp__postman__*` 工具时，Step 2 逐接口 curl 结束后可以额外用 MCP 跑 `api/postman/` 集合，把断言结果并入报告；没有就跳过，结论不变。Step 3 规定了分歧怎么判：MCP 断言失败而 Step 2 的 curl 通过，那是集合过期，记「集合待更新」，不记接口失败。报告模板里 Postman 集合一行的后半段是「MCP 运行结果: 未使用 / N 通过 M 失败」。curl 是主的，MCP 是附件。

第三个位置是 Step 6.2 用户验证。三选一：有 Postman MCP 就由 AI 代跑集合并展示结果，没有就手动导入集合，或者直接用 `api-verify` 报告里的 curl 命令。这一节的原话是「用户验证不是可选步骤，Postman 只是可选工具」。用户确认之前，Step 6 不算完成。自验通过之后为什么还要等这一步，见「验证闭环」。

1.0.0 演示版本走的就是没有 Postman MCP 的路径。`api-verify` 按 Skill 对真实进程核对了 17 项场景，全部通过，场景清单见「验证闭环」；报告里「MCP 运行结果」字段填的是「未使用」。

集合本身照常维护。`api/postman/go-backend-template.postman_collection.json` 目前有 7 个请求：

| 目录 | 请求 |
| --- | --- |
| 健康检查 | `GET /healthz` |
| v1 | `GET /v1/ping`、`POST /v1/ping (405)` |
| notes（1.0.0） | `POST /v1/notes 创建笔记`、`GET /v1/notes/{id} 读取笔记`、`GET /v1/notes/{id} 不存在 (404)`、`POST /v1/notes title 为空 (400)` |

集合变量 `baseUrl` 默认 `http://localhost:8080`，`noteId` 由创建请求的测试脚本写入。它是 Step 7 的交付物之一。`spec-reviewer` 第一轮的 R5 把 README 与技术讲解落后于代码列为 finding，集合与契约同步是同一类要求。集合被谁运行、有没有被 MCP 运行，不进入任何门禁。

## 能力前置校验表怎样分级，STOP 规则怎样生效

表在 `.ai/ai-rules.md` 的「能力前置校验」一节。能力名里带「可选」的行就是可选能力：

| 能力 | 探针 | 缺失时 | 分级 |
| --- | --- | --- | --- |
| Go 工具链 | `go version` 输出版本不低于 go.mod 声明 | STOP，提示用户安装或切换 Go 版本 | 必需 |
| make | `make -v` | STOP，提示安装（或改用 Makefile 内等价命令并说明） | 必需 |
| curl | `curl --version` | 冒烟与 api-verify 不可用，提示安装 | 必需 |
| Skill | 系统提示中的可用 Skill 列表 | 所需 Skill 不在列表中视为不可用，告知用户 | 必需 |
| git（可选） | `git --version` | 只影响 `init-project` 的 git init 与 `sync-ai-assets` 的 git clone；缺失时跳过对应步骤并告知 | 可选 |
| Postman MCP（可选） | 系统提示中存在 `mcp__postman__*` 工具 | 不可用时不阻塞：用户验收改用手动导入集合或 curl | 可选 |
| golangci-lint / govulncheck（可选） | `command -v` | 不可用时 `make lint` / `make vuln` 自动跳过 | 可选 |

表后面的三句话才是规则本体。必需能力校验失败必须 STOP，告知缺什么、怎么装，不得绕过或用别的方式凑合。可选能力缺失只需说明改用什么替代。本工程的必需门禁不依赖任何 MCP Server。校验在同一会话内通过后不重复，命令失败则视为失效重新校验。它是每次门禁前的前提，不是开场仪式。

可选行的「自动跳过」直接写在 `Makefile` 里：

```make
# Makefile
lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run --timeout 5m || echo "golangci-lint 未安装，跳过（可选）"

vuln:
	@command -v govulncheck >/dev/null 2>&1 && govulncheck ./... || echo "govulncheck 未安装，跳过（可选）"
```

门禁记录里，`make lint` 与 `make vuln` 因为两个工具未安装按设计跳过，其余五道门禁全部通过。可选能力缺失被如实记下，既没写成失败，也没写成通过。

Postman MCP 停在可选行，根源是 Step 6.1 的一句话：「测试是验证的主体：手动 curl 或 Postman 通过但没有对应测试的行为，不算已验证。」`api-verify` 的 Step 3.5 还要求每个场景都能找到对应的 `*_test.go` 或 `*_integration_test.go` 用例，找不到就记「缺测试」，补齐前不能给「通过」。MCP 代跑一次全绿，也只是断言结果，替代不了任何一个用例。每条验收项对应 Case，说的是同一件事。

## 业务工程要接 MCP 时放在哪里

模板不禁止 MCP，只规定它进不了门禁。从模板初始化的业务工程真要接某个 MCP，按下面四条处理。

第一，配置分 Agent 存放。Claude Code 的 MCP 配置放项目根目录的 `.mcp.json`，Codex 的放 `.codex/config.toml` 的 `[mcp_servers]` 表。这两样当前模板里都没有。Skills 靠 `.claude/skills` 与 `.agents/skills` 两个符号链接共用 `.ai/skills`，MCP 配置没有这种单源，两个文件各写各的。文件权限规则里，`.codex/config.toml` 属于 AI 可修改但「要在变更说明中告知用户」的一类。`.mcp.json` 还不在这份清单里，加进去时按同一条规则处理。密钥不写入代码和文档，沿用「日志与配置」一节的约束。

第二，在能力前置校验表加一行探针。写法跟 Postman 行一致：能力名带「可选」，探针写「系统提示中存在 `mcp__<name>__*` 工具」，缺失时写清替代方式。`.ai/ai-rules.md` 允许随项目规则演进更新，只是「文件权限规则」「AI 工作流」「验证」三节的约束不能删。

第三，验证命令表不动。`make check`、`make test-integration`、`make test-race`、`make smoke`、`api-verify` 的必需地位不会因为多了一个 MCP 就变。哪怕 MCP 面向的是数据库这类外部依赖，Step 6.1 第 2 条的要求还是先把依赖准备好，连接串环境变量、`/readyz` 都要到位，由 `make test-integration` 用真实依赖覆盖成功与失败路径，不得因依赖未就绪跳过。MCP 能帮 Agent 查看数据，证据仍来自测试。

第四，同步时它不会被覆盖。`sync-ai-assets` 的对比清单里没有 `.mcp.json`，同步不碰它。`.codex/config.toml` 在同步时是「展示差异，用户决定」，业务工程自己加的 `[mcp_servers]` 不会被模板版本静默替换。

## MCP、Skill、Subagent、Hook 各解决什么

四种机制在这份模板里各占各的位置，边界放进一张表更好对照：

| 机制 | 本模板中的实例 | 解决什么 | 不解决什么 | 适用 Agent |
| --- | --- | --- | --- | --- |
| MCP | 无配置；Postman MCP 只是可选探针 | 让 Agent 接入外部工具或服务 | 不产生门禁证据；缺失不阻塞 | Claude Code 用 `.mcp.json`，Codex 用 `[mcp_servers]`，各自配置 |
| Skill | `init-project`、`spec-coding-init`、`api-verify`、`sync-ai-assets`、`spec-graph-workflow`、第三方 `go-development` | 把一段工作流的步骤、命令、STOP 条件固定下来 | 不限制工具权限；结果仍来自命令 | 两端共享，`.claude/skills` 与 `.agents/skills` 都指向 `../.ai/skills` |
| Subagent | `spec-reviewer`（tools: Read, Grep, Glob）、`spec-implementer`（tools: Read, Grep, Glob, Edit, Write, Bash） | 收窄上下文与工具面：只读审查、按切片实现 | 结论不替代 `make check` 等命令结果；Codex 无对应机制 | Claude Code |
| Hook | `PostToolUse` 匹配 `Edit` 与 `Write`，运行 `scripts/check-format.sh` | Edit / Write 之后即时 gofmt 反馈 | 事后反馈，不能撤销写入，不覆盖 Bash 改动，不是安全边界 | Claude Code |

`.ai/ai-rules.md` 在这张分工的末尾给了统一的底线：真正需要禁止的操作依靠权限、沙箱和人工确认，不依靠上述任一项。MCP 站在同一侧，它扩展的是能力面，管不到安全面。

四种机制在 1.0.0 演示里的验证程度不一样，得如实分开。`spec-reviewer` 两轮审查返回 6 条 finding，全部 verified；实现由通用 Agent 加载 `.claude/agents/spec-implementer.md` 正文完成。两个 Subagent 的 Claude Code 原生 agents 目录加载、运行时工具限制，都没有端到端记录。`scripts/check-format.sh` 的脚本行为已用探针验证，真实 Claude Code 会话里 `PostToolUse` 的自动触发还没有记录，两条边界见「Subagent 与 Hook 落地」。Postman MCP 从头到尾没有出现过。五道门禁的通过记录不依赖其中任何一项，它们都留在辅助层。Skill 的细节见「Skill 落地」，Subagent 与 Hook 见「Subagent 与 Hook 落地」。

## 证据绑定的是候选，不是工具

`Specs/technical/1.0.0/graph.json` 里，`api-verify` 作为一类证据被 `record` 了三次：第一次绑定候选 `04a68378d8b4`，审查 `changes_required` 修复之后两次绑定候选 `2d03ec4f1c56`。每条记录存的是退出码、日志的 SHA-256 摘要、当时的候选摘要与 inputs 摘要。哪怕某次运行里 Postman MCP 可用，它的断言结果也只进报告的「MCP 运行结果」字段，`record` 的仍是 `api-verify` 这一类证据。Graph 只问这条结论在哪份代码上成立、之后有没有失效，不问当时用了什么工具。Loop 管局部收敛，Graph 管结论是否仍有效，理论部分见「Spec + Graph 理论」，CLI 与 graph.json 的落地见「Spec + Graph 落地」。

1.0.0 停在 `stage: verifying`、`user_acceptance: pending`、`review: pass`，revision 46。`event 1.0.0 verified` 被守卫拒绝，理由是「技术方案「交付状态」user_acceptance 为 pending，需要 confirmed」，退出码 3。有没有 Postman MCP 都一样，这道守卫只看用户是否确认。

## 下一个会话拿到这份模板该怎么判断

- 没有看到 `.mcp.json`：正常状态，不是缺配置。按能力前置校验表先跑 `go version`、`make -v`、`curl --version`。
- 系统提示里有 `mcp__postman__*`：可以在 `api-verify` Step 2 之后附加跑集合，结果写进报告的「MCP 运行结果」字段；与 curl 结果不一致时以 curl 为准，集合记「待更新」。
- 没有：报告写「未使用」，6.2 让用户手动导入集合或用 curl。1.0.0 就是这样走完的，仍在等用户确认。
- 要加 MCP：两个配置文件各放各的，校验表加一行可选探针，验证命令表不动，变更说明里告知用户。

必需门禁只依赖本机能装的三样东西，为的是让「通过」这两个字在任何一台机器上都能重新验证一遍。MCP 让 Agent 能做更多事，可它做的事想变成结论，还是要过同一组命令。
