# go-backend-template

最小生产级 Go 后端模板工程，仅依赖标准库，内置 Spec Coding 开发范式（Claude Code 与 Codex 双 Agent）：人工需求只读、技术方案确认后编码、测试分层为必需门禁、`spec-check` 机器核对文档一致性、用户确认后才算交付；可选启用 `spec-graph` 状态图与 Subagent 编排。

## 快速开始

```
make run               # 启动服务，默认 :8080
make check             # vet + 单元测试 + gofmt + spec-check（必需）
make spec-check        # 单独运行 Spec 一致性检查（版本目录、Feature ID、技术方案章节、交付状态、符号链接）
make spec-init VERSION=x.y.z   # 人工需求已存在时，由模版生成该版本技术方案
make test-integration  # 集成测试，-tags integration（必需）
make test-race         # 竞态检测（并发改动时必需）
make smoke             # 编译真实二进制并 curl 冒烟
make lint / make vuln  # golangci-lint / govulncheck，未安装自动跳过（可选）
make build             # 编译到 bin/api
```

从模板新建业务工程：在模板目录中运行 Skill `/init-project`（Codex 为 `$init-project`），按提示输入目录名与 Go module 路径即可；也可以手动执行（脚本兼容 macOS BSD sed 与 Linux GNU sed）：

```bash
bash .ai/skills/init-project/rename_module.sh <新目录名> <module路径>
```

## 配置（环境变量）

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `HTTP_ADDR` | `:8080` | 监听地址 |
| `SHUTDOWN_TIMEOUT` | `10s` | 优雅停机超时 |

## 路由

- `GET /healthz` 健康检查，返回 `{"status":"ok"}`
- `GET /v1/ping` 示例接口，返回 `{"message":"pong"}`
- `POST /v1/notes` 创建笔记（1.0.0 演示版本，内存存储），`{"title","content"}` -> 201 `{"id","title","content","created_at"}`
- `GET /v1/notes/{id}` 读取笔记，不存在返回 404 `not_found`

错误统一返回 `{"error":{"code":"...","message":"..."}}`，错误码约定见 `Specs/requirements/协议与数据.md`。

本地验收（可选工具）：`make run` 后导入 `api/postman/go-backend-template.postman_collection.json` 到 Postman（或由 Postman MCP 代跑），或直接 `curl localhost:8080/v1/ping`。验证的主体是单元测试与集成测试，Postman 只用于用户验收。

## Spec Coding 开发范式

```
Specs/
├── requirements/          人工维护，AI 只读
│   ├── 需求模版.md        新版本需求从此复制到 {version}/需求.md
│   └── 协议与数据.md      接口通用约定、错误码、对接方约束
└── technical/             AI 维护，始终与代码一致
    ├── 技术讲解.md        项目技术全景
    └── 技术方案模版.md    新版本方案从此复制到 {version}/技术方案.md
```

版本开发流程：人工在 `Specs/requirements/{version}/需求.md` 写需求（功能编号 `F-{version}-NNN`）→ AI 读取上下文并确认歧义 → API 契约设计 → `make spec-init VERSION={version}` 生成技术方案并填写，用户确认后「交付状态」改 `implementing` → 测试先行编码（单元 + 集成）→ 自验（`make check`、`make test-integration`、`make test-race`、`make smoke`、`api-verify`；可选委派只读 Subagent `spec-reviewer`）→ 用户用 Postman / curl 验证并确认，「交付状态」`user_acceptance` 改 `confirmed` → 更新技术方案、技术讲解、Postman 集合，「交付状态」改 `delivered`。`spec-check` 会拒绝未经用户确认的 `delivered`。完整规则见 `CLAUDE.md` / `AGENTS.md`（均指向 `.ai/ai-rules.md`）。

`Specs`、`.ai`、`.claude`、`.codex`、`.agents`、`api` 等工作流目录在 macOS Finder 中标了红色标签，便于和业务代码目录区分。标签存放在文件系统扩展属性中：`rename_module.sh` 用 tar 复制时会保留，git clone 不会携带，需要时在 Finder 中重新标记即可。

### 双 Agent 支持

| 路径 | 说明 |
| --- | --- |
| `.ai/ai-rules.md` | 行为准则单源 |
| `.ai/memory.md` | 工程记忆：已验证的事实、踩过的坑、待验证，跨会话继承 |
| `.ai/skills/` | 共享 Skills |
| `CLAUDE.md`、`.claude/` | Claude Code：符号链接到 ai-rules，`skills -> ../.ai/skills`；`settings.json` 预授权 go、gofmt、make、curl、git、bash、lsof、shasum，并配置 `PostToolUse` Hook 在 Edit / Write 后运行 `scripts/check-format.sh`；`agents/` 下是只读审查 `spec-reviewer` 与实现者 `spec-implementer` 两个 Subagent |
| `AGENTS.md`、`.codex/`、`.agents/` | Codex：符号链接到 ai-rules，`skills -> ../.ai/skills` |

### Skills

| Skill | 用途 |
| --- | --- |
| `init-project` | 从模板复制并初始化业务工程 |
| `spec-coding-init` | 将已有 Go 工程改造为 Spec Coding 模式 |
| `api-verify` | 启动服务，按技术方案逐接口 curl 验证并出报告 |
| `sync-ai-assets` | 从模板同步 Skills、行为准则、Specs 模版、配置 |
| `spec-graph-workflow`（可选） | 用 `spec-graph` 状态图与 Subagent 展开一个版本的实现、审查、验证与失效追踪，见 `docs/spec-graph/` |
| `go-development` | 第三方（netresearch，MIT AND CC-BY-SA-4.0）Go 工程实践参考，来源与适用范围见 `.ai/skills/go-development/SOURCE.md` |

## 目录结构

```
cmd/api/             入口：配置加载、HTTP server、优雅停机
cmd/spec-check/      Spec 一致性检查（make check 的一部分）
cmd/spec-graph/      可选：版本生命周期状态图 CLI
internal/config/     环境变量配置
internal/httpapi/    路由、handler（含 notes）、统一响应、请求日志与 panic 恢复中间件
internal/note/       笔记领域：规则校验、ID 生成、Repository 端口（1.0.0 演示）
internal/memstore/   note.Repository 的内存实现
internal/specdoc/    解析技术方案「交付状态」块（spec-check 与 spec-graph 共用）
internal/speccheck/  spec-check 规则实现
internal/specgraph/  状态图、守卫、证据摘要与 graph.json 读写
api/postman/         Postman 集合
scripts/             smoke.sh 冒烟；spec-init.sh 生成版本技术方案；check-format.sh 供 Hook 调用
docs/spec-graph/     可选 spec+graph 工作流的理论与落地说明
Specs/               需求与技术文档
.ai/                 AI 资产单源
```

### 可选：Spec + Graph 工作流

版本较大、需要独立审查或需要在需求 / 代码变化后判断哪些证据已失效时，运行 Skill `spec-graph-workflow`：`go run ./cmd/spec-graph init {version}` 建立 `Specs/technical/{version}/graph.json`，主会话作为唯一 Controller 运行门禁并 `record` 证据、登记 `finding`、用 `event` 推进 planning → implementing → reviewing → fixing → verifying → ready_to_deliver；实现与审查分别委派给 Subagent `spec-implementer` 与 `spec-reviewer`。用户决定只记录在技术方案「交付状态」块，Graph 读取它作为守卫，不另存一份完成真相。理论与命令说明见 `docs/spec-graph/理论.md` 与 `docs/spec-graph/落地.md`。

按需再加 `internal/{domain}/`（业务规则与存储端口）、`internal/postgres/`（存储实现）、`migrations/`（数据库迁移），依赖方向只允许 `cmd -> httpapi -> domain <- storage`，不预置空抽象。
