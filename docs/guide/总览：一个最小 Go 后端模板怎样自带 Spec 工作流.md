---
title: 总览：一个最小 Go 后端模板怎样自带 Spec 工作流
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - overview
  - spec-coding
  - go-backend
---

# 总览：一个最小 Go 后端模板怎样自带 Spec 工作流

`go-backend-template` 只依赖标准库。`go.mod` 声明 `go 1.25.0`，module 是 `example.com/go-backend-template`，本机验证运行时为 Go 1.27.0 darwin/arm64。四条路由、环境变量配置、`slog` 结构化日志、优雅停机、统一错误信封，第三方依赖为零。

同一个目录里还放着一套版本化的 Spec 工作流，由机器守着：人工需求只读；技术方案确认后才编码；单元与集成测试是必需门禁；`make check` 里的 `spec-check` 核对文档一致性；`delivered` 要等用户确认才能写。Claude Code 与 Codex 共读同一份行为准则，Hook 与 Subagent 只在 Claude Code 下生效。

下面每一条落点都能在仓库里打开核对，`1.0.0` 演示版本的每一步结果来自真实运行。

先说清楚它的分量：这是一个 MVP，给的是形状和边界，不是成品工具链。

## 骨架、范式、双 Agent 各是什么

骨架只有三个基础包：`cmd/api`、`internal/config`、`internal/httpapi`，再加 `1.0.0` 演示用的 `internal/note` 与 `internal/memstore`。对外接口是 `GET /healthz`、`GET /v1/ping`、`POST /v1/notes`、`GET /v1/notes/{id}`。依赖方向只允许 `cmd -> httpapi -> domain <- storage`，不预置空抽象。

范式落在 `Specs/` 下的两个目录和 `.ai/ai-rules.md` 里的七个 Step。`Specs/requirements/` 归人工维护，AI 只读；`Specs/technical/` 归 AI 维护，始终与代码同步。每个版本的技术方案有一个固定章节「交付状态」，位置是倒数第二节，其后只有「变更记录」。这一节里是机器可读的 YAML 块，只含 `stage`、`user_acceptance`、`review` 三个键。用户决定在整个仓库里只有这一个落点。谁维护哪份文件，先于任何一行代码定下来。

双 Agent 靠符号链接做单源。`CLAUDE.md` 与 `AGENTS.md` 都指向 `.ai/ai-rules.md`，`.claude/skills` 与 `.agents/skills` 都指向 `../.ai/skills`。`.codex/config.toml` 里只有 `personality` 和注释，没有 `mcp_servers`，仓库也没有 `.mcp.json`。入口文件负责把会话带到规则，自身不装内容。

## 目录全景

```text
go-backend-template/
├── cmd/api/                 入口：配置、http.Server、优雅停机
├── cmd/spec-check/          Spec 一致性检查，make check 的一部分
├── cmd/spec-graph/          可选：版本生命周期状态图 CLI
├── internal/config/         环境变量配置（HTTP_ADDR、SHUTDOWN_TIMEOUT）
├── internal/httpapi/        路由、handler、统一响应、中间件、单元与集成测试
├── internal/note/           笔记领域：规则、ID 生成、Repository 端口（1.0.0）
├── internal/memstore/       note.Repository 的内存实现
├── internal/specdoc/        解析「交付状态」块，speccheck 与 specgraph 共用
├── internal/speccheck/      spec-check 十条规则
├── internal/specgraph/      graph.go、digest.go、store.go、transition.go、check.go
├── api/postman/             Postman 集合，7 个请求
├── scripts/                 smoke.sh、spec-init.sh、check-format.sh
├── docs/spec-graph/         可选 spec+graph 工作流说明的目录
├── Specs/requirements/      需求模版.md、协议与数据.md、1.0.0/需求.md
├── Specs/technical/         技术讲解.md、技术方案模版.md、1.0.0/技术方案.md、1.0.0/graph.json
├── .ai/                     ai-rules.md、skills/（五个自研 + go-development）
├── .claude/                 settings.json、agents/、skills -> ../.ai/skills
├── .codex/config.toml       Codex 配置
├── .agents/skills -> ../.ai/skills
├── CLAUDE.md -> .ai/ai-rules.md
├── AGENTS.md -> .ai/ai-rules.md
└── Makefile                 run / build / test / test-integration / test-race / fmt / vet / check / spec-check / spec-init / lint / vuln / smoke / clean
```

`Specs`、`.ai`、`.claude`、`.codex`、`.agents`、`api`、`docs`、`scripts`，连同它们下面的版本目录、`skills`、`agents` 等子目录，共 16 个目录在 macOS Finder 里标了红色标签，用来和业务代码区分。标签存在扩展属性里，`rename_module.sh` 用 tar 复制时保留，git clone 不携带。

## 七类机制各落在哪个文件

| 机制 | 落点 | 守住什么 |
| --- | --- | --- |
| Rules | `.ai/ai-rules.md`；`CLAUDE.md`、`AGENTS.md` 为符号链接 | 文件权限、七个 Step、代码规范、验证命令的唯一来源；`spec-check` 规则 2 核对它的三个必需章节，规则 4 核对四条符号链接 |
| Spec | `Specs/requirements/`（人工）、`Specs/technical/`（AI）、技术方案「交付状态」块 | 两侧版本目录一一对应，每个 `F-{version}-NNN` 都出现在技术方案；用户决定只记录在交付状态块 |
| Command | `Makefile` 目标、`scripts/`、`cmd/spec-check`、`scripts/spec-init.sh`、`cmd/spec-graph`（可选） | `make check` 是必需门禁；`spec-check` 十条规则拒绝不一致文档与未经确认的 `delivered` |
| Skill | `.ai/skills/`：`init-project`、`spec-coding-init`、`api-verify`、`sync-ai-assets`、`spec-graph-workflow` 五个自研，`go-development` 一个第三方 | 每个 Skill 接管生命周期的一个时刻；第三方目录只读，来源、许可证与 26 个上游文件的 SHA-256 记录在 `SOURCE.md` |
| Subagent | `.claude/agents/spec-reviewer.md`（`Read, Grep, Glob`）、`spec-implementer.md`（`Read, Grep, Glob, Edit, Write, Bash`） | 审查只读，实现只跑 focused 测试；两者结论都不替代命令结果 |
| Hook | `.claude/settings.json` 的 `PostToolUse`，matcher `Edit\|Write` -> `scripts/check-format.sh` | 编辑后立即 gofmt 检查 `cmd/` 与 `internal/`；事后反馈，不覆盖 Bash 改动，不是安全边界 |
| MCP | 无必需 MCP；Postman MCP 可选，探针是系统提示中是否存在 `mcp__postman__*` 工具 | 必需门禁全部走本地 Go 工具链与 curl，Postman 只是用户验收工具 |

`settings.json` 另有 8 条 `permissions.allow` 预授权：go、gofmt、make、curl、git、bash、lsof、shasum。预授权只减少确认次数，不改变文件权限规则。

## 1.0.0 从人工需求到交付状态的最短路径

**Step 1 到 Step 4：需求、契约与方案。** `Specs/requirements/1.0.0/需求.md` 由 Agent 在用户授权下代写，落盘后按人工需求只读。里面定义两个功能：`F-1.0.0-001` 创建笔记 `POST /v1/notes`（AC-1 到 AC-5），`F-1.0.0-002` 读取笔记 `GET /v1/notes/{id}`（AC-1 到 AC-3），数据只放内存。只创建需求就跑 `make spec-check`，输出 `spec-check: 1 problem(s)`，提示执行 `make spec-init VERSION=1.0.0`；执行后输出 `created: Specs/technical/1.0.0/技术方案.md`；把「需求摘要」填好、引用两个 Feature ID，才得到 `spec-check ok (1 version(s))`。`scripts/spec-init.sh` 的负例也跑过：需求不存在退出码 1，版本号写成 `v1` 退出码 2。方案确认来自用户会话授权，记录在技术方案「变更记录」，`stage` 改为 `implementing`。

**Step 5：委派实现。** 实现由 `spec-implementer` 角色完成，执行方式是通用 Agent 加载 `.claude/agents/spec-implementer.md` 正文，不走 Claude Code 原生 `agents/` 目录加载。它只跑 focused 测试，返回 11 个改动文件和每个用例的 RED/GREEN 记录。主会话作为唯一 Controller 跑 broad 门禁并记录证据。

**Step 6.1：门禁与审查。** `make check`（`go vet`、`go test ./...`、`gofmt -l`、`spec-check`）、`make test-integration`、`make test-race`、`make smoke`（9 项检查）全部通过。`make lint` 与 `make vuln` 因 golangci-lint、govulncheck 未安装按设计跳过。`api-verify` 按 Skill 用 `mktemp` 目录和端口 18090 启动真实进程，17 项场景全过，服务器日志不含笔记正文，端口释放、运行目录删除。没有 Postman MCP 在场，7 个请求的集合未由 MCP 代跑。

`spec-reviewer` 第一轮 verdict 是 `changes_required`，给出 R1 到 R5 五条 finding：代码缺陷两条，R1 `Store.Save` 静默覆盖、R2 `decodeJSON` 不查尾随数据；文档偏差三条，R3 到 R5。R1 与 R2 由 `spec-implementer` 角色测试先行修复，R3 到 R5 由 Controller 回写文档。第二轮 verdict 为 `pass`，新增 R6 P3，测试计划一个用例名挂错。六条 finding 全部 `verified`。逐条内容与修复方式见「Subagent 与 Hook 落地」。

**Graph 记录了什么。** 这个版本启用了可选的 `spec-graph`。`Specs/technical/1.0.0/graph.json` 的 revision 计到 46，`init` 为 revision 0：`init` 到 `plan_confirmed`，第一轮门禁绑定候选 `04a68378d8b4`，审查失败进入 `fixing`，修复后候选变成 `2d03ec4f1c56`，第二轮审查通过进入 `verifying`，最终门禁绑定候选 `2d03ec4f1c56` 与 inputs `1c1fd583b8f5`。逐 revision 的表见「Spec + Graph 落地」。

修复轮之后门禁全部重新记录，候选摘要已经从 `04a68378d8b4` 变成 `2d03ec4f1c56`。文档回写后 inputs 摘要也变了，最终一轮在 `1c1fd583b8f5` 上再记一次。门禁证据绑定的是候选身份。`go run ./cmd/spec-graph check 1.0.0` 退出码 0；接着执行 `event 1.0.0 verified` 被守卫拒绝，退出码 3：

```text
守卫不满足: verified: 技术方案「交付状态」user_acceptance 为 pending，需要 confirmed
```

**停在哪里。** graph stage `verifying`、revision 46、`invalidated: false`；技术方案「交付状态」是 `stage: verifying`、`user_acceptance: pending`、`review: pass`。门禁全通过，审查也通过，版本仍未交付。绿色不等于交付，还差一个只有用户能写的字段。

确认之后的动作是固定的：把 `user_acceptance` 改为 `confirmed`，执行 `event 1.0.0 verified` 进入 `ready_to_deliver`，按 Step 7 的七项回写技术方案、技术讲解、Postman 集合、`scripts/smoke.sh`、README 与技术方案「测试计划」，把 `stage` 改为 `delivered`，最后再跑一次 `make check` 与 `go run ./cmd/spec-graph check 1.0.0`。`spec-check` 规则 8 会拒绝 `user_acceptance` 未 `confirmed`、或 `review` 为 `pending` / `changes_required` 的 `delivered`。

最后一步不保证退出码 0。`user_acceptance` 与 `stage` 都在「交付状态」块内，inputs 摘要剔除该块，改它们不漂移。技术方案正文计入 inputs 摘要，Step 7 第 1 项要求更新的变更记录就写在正文里；`api/` 与 `scripts/` 计入 candidate 摘要。回写一旦触及这些位置，`spec-graph check` 在 `ready_to_deliver` 下报「readiness 已失效」，退出码 1。按 `spec-graph-workflow` Skill 的 Step E，执行 `event 1.0.0 readiness_invalidated` 回到 `fixing`，「交付状态」`stage` 改回 `implementing`，重新记录证据再往前推。技术讲解与 README 不在两份摘要内，改它们不受影响。

## 哪些事已验证，哪些事还没有端到端记录

Hook 的脚本行为已经探针验证：放入未格式化的 `internal/hookprobe/unformatted.go`，从仓库外执行 `scripts/check-format.sh`，输出 `gofmt needed:` 与该路径、退出码 2；删掉探针后退出码 0。真实 Claude Code 会话里 `PostToolUse` 的自动触发还没有记录。两个 Subagent 的 frontmatter 已配置 `tools`，Claude Code 原生加载与运行时工具限制同样没有端到端记录，`1.0.0` 的实现是通用 Agent 加载 `spec-implementer.md` 正文完成的。

门禁从不依赖 Hook 或 Subagent，这些缺口不影响门禁结论：`make check` 会独立跑 gofmt，审查结论也从不替代命令结果。

## 边界：这是一个 MVP

模板里的规则、Skill、Subagent、Hook 和 MCP 约定，都停在「一个通用 Go HTTP 服务需要的最基础工作流」这条线上。没有业务，没有数据库，没有发布流程，没有任何一家公司的约定。

各机制当前的覆盖范围与真实项目会缺的部分：

| 机制 | 模板给到哪里 | 落到业务上通常还要补 |
| --- | --- | --- |
| 规则 | 文件权限、七个 Step、代码规范、DRY 红线 | 团队技术选型边界、分支与发布约定、领域术语表、安全与合规条款 |
| Skill | 初始化、存量接入、接口验证、资产同步、可选状态图 | 部署与灰度、数据迁移、故障排查、性能压测、领域专属的评审清单 |
| Subagent | 只读审查、委派实现 | 领域审查角色（安全、数据一致性、成本），以及跨服务改动的影响面分析 |
| Hook | 编辑后 gofmt 提醒 | 提交前的密钥扫描、生成代码校验、依赖许可证检查 |
| MCP | 一个都不需要，Postman 可选 | 数据库、内部服务、监控与日志平台、工单系统的连接 |
| 门禁 | `make check`、集成、race、冒烟、契约核对 | 覆盖率阈值、性能基线、漏洞扫描、镜像构建与部署验证 |

这些留白是有意的。哪些流程值得固化成 Skill、哪些外部系统值得接 MCP、哪道检查值得加进门禁，取决于团队日常在什么地方反复出错、反复返工。照搬一套别人的清单只会得到一堆没人用的文件。

用法是：先按模板跑通一个真实版本，记下哪一步靠人反复提醒才没做错，再把那一步固化成规则或 Skill。`.ai/skills/` 与 `.claude/agents/` 的目录结构、`spec-check` 的规则写法、`spec-graph` 的守卫合同都可以直接照着扩展。

## 后续 16 篇各回答什么问题

| 篇 | 回答的问题 |
| --- | --- |
| `Rules 落地：ai-rules.md 单源怎样被两个 Agent 共读并被机器守住` | 一份行为准则怎样通过符号链接与 `spec-check` 规则同时约束 Claude Code 和 Codex |
| `Spec 落地：requirements 与 technical 两个目录各归谁维护` | 需求、协议、技术方案、交付状态块的所有权与机器核对 |
| `spec-init：一个新版本怎样从人工需求开始` | `scripts/spec-init.sh` 的前提、产物、拒绝条件与退出码 |
| `版本开发工作流：七个 Step 每一步的输入、输出和停止条件` | 从读取上下文到维护文档，哪一步必须停下来等用户 |
| `API 契约设计：接口在编码前怎样被冻结` | Step 3 契约、`协议与数据.md` 的约定与破坏性变更的判断 |
| `Command 落地：make 目标、脚本与 spec-check 各自守什么` | `Makefile` 每个目标的内容与 `spec-check` 十条规则 |
| `测试分层：单元、集成、race、冒烟各自证明什么` | 四层测试的构建标签、运行命令与各自能证明的范围 |
| `验证闭环：自验通过之后为什么还要等用户确认` | 6.1、6.2、6.3 与 `user_acceptance` 字段的关系 |
| `Skill 落地：五个自研 Skill 分别接管生命周期的哪个时刻` | `init-project`、`spec-coding-init`、`api-verify`、`sync-ai-assets`、`spec-graph-workflow` 各自的触发点 |
| `第三方 Skill：go-development 是怎样被引入、约束和升级的` | `SOURCE.md` 记录的来源、许可证、完整性摘要、冲突处理与升级方式 |
| `Subagent 与 Hook 落地：只读审查、委派实现与编辑后的格式检查` | 两个 Subagent 的交接合同与 `PostToolUse` Hook 的边界 |
| `MCP 落地：为什么必需门禁不依赖任何 MCP` | Postman MCP 的可选定位与能力前置校验 |
| `代码规范与日常修改：不走版本流程的改动怎样不破坏一致性` | 包组织、错误处理、DRY 红线与日常修改的最小流程 |
| `工程骨架：cmd 与 internal 里每个文件在做什么` | 入口、配置、HTTP 层、领域层、存储层与三个工具包的逐文件说明 |
| `Spec + Graph 理论：Loop 管局部收敛，Graph 管结论是否仍然有效` | 为什么需要在 Loop 之上再加一层证据失效判断 |
| `Spec + Graph 落地：spec-graph CLI、graph.json 与 Subagent 编排` | 六个阶段、七类事件、守卫、摘要规则、退出码与 Controller 编排 |

本篇能回答三件事：这个模板守住了什么，谁在守，`1.0.0` 为什么停在 `verifying`。后面每一篇只打开其中一个落点。
