# AI Coding 行为准则

## 项目简介

这是 go-backend-template 模板工程：最小生产级 Go 后端服务骨架，采用 Spec Coding 驱动的开发模式。从本模板初始化业务工程后，本节应改为业务描述（运行 Skill `init-project` 完成）。

- **语言**：Go（module 声明 `go 1.25.0`）
- **依赖**：仅标准库；新增第三方依赖需先告知用户
- **架构**：`cmd`（入口）→ `internal/httpapi`（HTTP 层）→ `internal/{domain}`（业务层）← `internal/{storage}`（存储层）
- **验证**：单元测试与集成测试为必需门禁（`make check` 含 `go vet`、单测、gofmt、`spec-check`；`make test-integration`），`make smoke` 真实进程冒烟；Postman（MCP 或手动）为可选的用户验收工具
- **可选扩展**：`spec-graph` CLI 与 Skill `spec-graph-workflow` 把版本生命周期编排为确定性状态图并用 Subagent 展开，见 `docs/spec-graph/`

## 目录结构

```
project-root/
├── Specs/
│   ├── requirements/               # [人 -> AI] 人工维护，AI 只读
│   │   ├── 需求模版.md             # 新版本需求从此复制
│   │   ├── 协议与数据.md           # 接口通用约定、错误码、对接方约束
│   │   └── {version}/需求.md       # 版本需求
│   └── technical/                  # [AI -> AI] AI 维护，始终与代码一致
│       ├── 技术讲解.md             # 项目技术全景
│       ├── 技术方案模版.md         # 新版本方案从此复制
│       └── {version}/技术方案.md   # 版本技术方案（含机器可读「交付状态」块）；启用 Graph 时同目录有 graph.json
├── cmd/api/                        # 服务入口
├── cmd/spec-check/                 # Spec 一致性检查，make check 的一部分
├── cmd/spec-graph/                 # 可选：版本生命周期状态图 CLI（init / status / record / finding / event / check）
├── internal/                       # 业务代码按职责分包；specdoc / speccheck / specgraph 是工具包
├── api/postman/                    # Postman 集合（AI 维护，与接口清单同步）
├── scripts/                        # smoke.sh 冒烟；spec-init.sh 生成版本技术方案；check-format.sh 供 Hook 调用
├── docs/spec-graph/                # 可选 spec+graph 工作流的理论与落地说明
├── .ai/                            # 共享 AI 资产（Claude Code 与 Codex 共用）
│   ├── ai-rules.md                 # 本文件（行为准则单源）
│   └── skills/                     # 自定义 Skills
├── .claude/                        # Claude Code：settings.json（预授权 + PostToolUse 格式检查 Hook）、agents/（spec-reviewer、spec-implementer）、skills -> ../.ai/skills
├── .codex/config.toml              # Codex 配置
├── .agents/skills -> ../.ai/skills # Codex Skills 符号链接
├── CLAUDE.md -> .ai/ai-rules.md
├── AGENTS.md -> .ai/ai-rules.md
├── Makefile
└── README.md
```

## 文件权限规则

- `Specs/requirements/**` — **AI 禁止修改**。这是人对 AI 的输入，由人工维护。需求有歧义时提问，不得替用户改需求。
- `Specs/technical/**` — **AI 负责维护**。必须反映代码最新状态。
- `api/postman/**`、`scripts/**` — AI 维护，接口变更时同步。
- `cmd/**`、`internal/**`、`go.mod`、`go.sum`、`Makefile` — AI 可新增和修改；改 Makefile 目标或依赖时同步 README 与技术讲解。
- `README.md` — AI 可修改，只在路由、配置、命令、Skills 变化时同步，不放进度或临时讨论。
- `.claude/settings.json`、`.claude/agents/**`、`.codex/config.toml`、`.gitignore` — AI 可修改，但新增预授权命令、Hook、Subagent 或忽略项要在变更说明中告知用户。
- `Specs/technical/{version}/graph.json` — 只能通过 `spec-graph` 命令写入，不手工编辑；它记录执行阶段与证据身份，不记录用户决定。
- `.ai/ai-rules.md` — 项目规则演进时可更新，但不得删除「文件权限规则」「AI 工作流」「验证」三节的约束。
- `.ai/skills/{自研 Skill}/**` — AI 可按真实需要修改，改动要在 Skill 内保持步骤自洽。
- `.ai/skills/{第三方 Skill}/**`（目录内含 `SOURCE.md`）— **AI 禁止修改**，只能整目录升级并更新 `SOURCE.md`。

---

## 能力前置校验

AI 需要执行构建、测试或接口验证前，先确认对应能力可用；同一会话内校验通过后不重复校验，遇到命令失败则视为失效重新校验。

| 能力 | 探针 | 缺失时 |
| --- | --- | --- |
| Go 工具链 | `go version` 输出版本不低于 go.mod 声明 | STOP，提示用户安装或切换 Go 版本 |
| make | `make -v` | STOP，提示安装（或改用 Makefile 内等价命令并说明） |
| curl | `curl --version` | 冒烟与 api-verify 不可用，提示安装 |
| Skill | 系统提示中的可用 Skill 列表 | 所需 Skill 不在列表中视为不可用，告知用户 |
| git（可选） | `git --version` | 只影响 `init-project` 的 git init 与 `sync-ai-assets` 的 git clone；缺失时跳过对应步骤并告知 |
| Postman MCP（可选） | 系统提示中存在 `mcp__postman__*` 工具 | 不可用时不阻塞：用户验收改用手动导入集合或 curl |
| golangci-lint / govulncheck（可选） | `command -v` | 不可用时 `make lint` / `make vuln` 自动跳过 |

必需能力校验失败必须 STOP 并明确告知缺什么、怎么装，不得绕过或用其他方式凑合。可选能力缺失只需说明改用的替代方式。本工程的必需门禁不依赖任何 MCP Server。

---

## AI 工作流（版本开发）

当用户要求开始某个版本的开发时，AI 必须按以下 Step 执行。

> **需求变更与新增规则**：开发过程中用户随时可能新增或修改需求。无论变更大小，AI 都必须：1) 重新读取需求文档，对比找出变更点；2) 更新技术方案中的相关章节（需求摘要、API 契约、模块设计、文件清单）；3) 向用户展示变更部分并确认后才能编码；4) 编码完成后同样执行 Step 6 和 Step 7。不得跳过「更新技术方案 -> 用户确认 -> 编码 -> 维护文档」。

### Step 1 — 读取上下文

按顺序完整读取：

1. `Specs/technical/技术讲解.md` — 当前技术状态
2. `Specs/requirements/{version}/需求.md` — 本版本需求
3. `Specs/requirements/协议与数据.md` — 接口通用约定与对接方约束
4. 上一版本 `Specs/technical/{prev}/技术方案.md`（如存在）— 了解最近变更

`{version}` 采用语义化三段版本号（如 `0.1.0`、`1.2.0`），`Specs/requirements/` 与 `Specs/technical/` 下的版本目录名与它完全一致；`{prev}` 是按语义版本排序后小于当前版本的最大目录。用户给出的版本号不符合该格式时先确认，不自行改写目录名。

### Step 2 — 理解需求

- 逐条理解每个功能的「做什么」「接口与数据」「业务规则与边界」「验收标准」
- **不明确、不完整或有歧义的需求必须向用户确认，不得自行假设**；典型歧义：幂等语义、并发冲突处理、分页与排序、字段可空性、错误码归属
- `技术讲解.md` 缺少相关模块说明时，先读代码补全理解
- 需求涉及新增第三方依赖（数据库驱动、消息队列、SDK）时在此步骤告知用户并取得同意

### Step 3 — API 契约设计

有接口新增或变更时执行；纯内部重构或无接口变更可跳过并说明。

1. 按 `协议与数据.md` 的约定，为每个接口写出：方法、路径、鉴权、请求体、成功响应、错误表（HTTP 状态码 + error.code + 触发条件）
2. 检查与已有接口的兼容性：字段删除、语义变化、状态码变化都属于破坏性变更，需升级路径版本前缀或经用户确认
3. 契约写入技术方案的「API 契约」一节，同时准备 Postman 集合的对应请求（Step 7 落盘）
4. 契约随技术方案一起交用户确认；确认后不得在编码中悄悄改变字段或状态码

### Step 4 — 出技术方案

1. 运行 `make spec-init VERSION={version}` 由模版生成 `Specs/technical/{version}/技术方案.md`（要求人工需求已存在，不覆盖已有文件），再按模版各节填写
2. 重点写清：模块划分与依赖方向、数据模型与迁移、测试计划、风险与回滚、文件清单；「需求摘要」必须列出需求中的每个 `F-{version}-NNN`
3. **技术方案必须向用户展示并确认后，才能开始编码**；确认后把「交付状态」块的 `stage` 改为 `implementing`
4. 启用可选 Graph 时，在此运行 `go run ./cmd/spec-graph init {version}` 与 `event {version} plan_confirmed`（见 Skill `spec-graph-workflow`）

### Step 5 — 写代码

- **测试先行**：每个业务规则和接口先写失败测试，确认失败原因正确，再写最小实现让测试通过
- **测试分层是必需交付物**，技术方案「测试计划」中列出的每一项都要落地：
  - 单元测试（无 tag，`make test`）：业务规则、校验、错误映射；HTTP 层用 `httptest` 驱动 handler
  - 集成测试（`//go:build integration`，`make test-integration`）：走真实网络的完整请求链路；接入数据库、缓存、外部服务时必须用真实依赖覆盖成功与失败路径
  - 涉及 goroutine、锁、连接池的代码补 `make test-race`
- 按技术方案的文件清单逐步实施，新增包按「代码规范 / 包组织」放置
- 编码时按需加载 Skill `go-development` 的 `references/testing.md`、`architecture.md`、`api-design.md`、`logging.md`；与本文件冲突时以本文件为准
- 动手前扫一遍现有公共实现（`internal/httpapi/response.go`、各领域包的端口接口），能复用不新造；见「代码规范 / DRY 红线」
- 涉及数据变更时先写迁移文件（含回滚），再写存储实现
- Claude Code 下每次 `Edit` / `Write` 后 `PostToolUse` Hook 会运行 `scripts/check-format.sh` 做 gofmt 即时提醒；它不覆盖 Bash 产生的改动，也不替代 `make check`

### Step 6 — 验证

#### 6.1 Agent 自验（必需）

按顺序执行，全部通过才算自验完成：

1. `make check`：`go vet`、单元测试、gofmt 检查、`spec-check`（版本目录与 Feature ID 一致性、技术方案必需章节、「交付状态」块、符号链接）
2. `make test-integration`：集成测试；本版本引入或触及外部依赖时必须先准备好依赖（连接串环境变量、`/readyz`），不得因依赖未就绪跳过
3. `make test-race`：本版本涉及并发改动时执行；无并发改动可说明后跳过
4. `make smoke`：真实二进制启动与基础接口检查
5. 契约核对：运行 Skill `api-verify`，按技术方案的 API 契约对真实进程逐接口 curl 校验状态码、响应字段、错误信封，覆盖成功路径与每种错误场景
6. 对照需求的「验收标准」逐条核对，列出核对结果，并确认每条标准都有对应的单元或集成测试固化
7. 进入 6.2 前把技术方案「交付状态」的 `stage` 改为 `verifying`

可选加强：`make lint`、`make vuln`（工具已安装时执行，结果附在报告中）。

可选独立审查（Claude Code）：自验通过后委派 Subagent `spec-reviewer`（`.claude/agents/spec-reviewer.md`，只读工具）按需求覆盖、实现正确性、生产风险三轮审查并返回 verdict 与 findings。`changes_required` 时对每条接受的 finding 先补失败测试再修，按 6.3 复验后再次审查；审查结论不替代命令结果。做了审查就把「交付状态」的 `review` 写成 `pending` / `changes_required` / `pass`，没做保持 `not_required`。Codex 没有 Subagent 机制时，可在新会话中按该文件正文执行同一审查。

> 自验中任一项失败，修复后重新执行受影响的项；不得带着失败进入 6.2。测试是验证的主体：手动 curl 或 Postman 通过但没有对应测试的行为，不算已验证。

#### 6.2 用户验证

1. 自验完成后，向用户展示验证结果（各命令输出摘要、接口核对表、验收标准与测试对应表）
2. 明确提示用户启动服务（`make run`）并按任一方式验收：
   - Postman：有 Postman MCP 时 AI 可代为运行 `api/postman/` 集合并展示结果；没有则用户手动导入集合
   - curl：使用 `api-verify` 报告中的命令
   - 用户反馈：接口行为与预期不符、响应字段 / 状态码 / 错误信息偏差、性能或数据问题
3. 用户验证不是可选步骤，Postman 只是可选工具。用户最终确认前，Step 6 不算完成，不得进入 Step 7
4. 用户明确确认后，把技术方案「交付状态」的 `user_acceptance` 改为 `confirmed`；启用 Graph 时随后执行 `event {version} verified`

#### 6.3 反馈修复循环

1. 用户反馈问题，AI 先补一个复现该问题的失败测试（单元或集成），再修改代码
2. 修改后重新执行受影响范围的自验：逻辑改动重跑 `make check` 与相关集成测试，接口改动重跑 `api-verify` 对应接口和 `make smoke`
3. 复验完成后再次进入用户验证模式，可能经历多轮「反馈 -> 修改 -> 复验 -> 用户验证」
4. 只有用户明确表示「确认验证通过」「没有问题」「可以进入下一步」等最终确认后，Step 6 才算完成
5. 用户反馈属于新增需求或需求变更时，按「需求变更与新增规则」处理，不得当作 bug 直接改

### Step 7 — 维护文档

1. `Specs/technical/{version}/技术方案.md` — 按实际实现回写，更新变更记录，保证与代码一致
2. `Specs/technical/技术讲解.md` — 更新目录结构、核心模块、接口清单、配置表、依赖列表、已知问题
3. `api/postman/*.postman_collection.json` — 新增 / 更新本版本接口的请求与断言（可选验收工具，但集合必须与契约一致）
4. `scripts/smoke.sh` — 关键新接口加入冒烟检查
5. `README.md` — 路由、配置、命令有变化时同步
6. 技术方案「测试计划」回写为实际落地的测试清单（文件与用例名），作为下一版本的回归基线
7. 技术方案「交付状态」改为 `stage: delivered`；`make check` 中的 `spec-check` 会拒绝 `user_acceptance` 未 `confirmed` 或 `review` 为 `pending` / `changes_required` 的 delivered

---

## 日常代码修改（非版本开发）

用户要求小范围修改（bug 修复、小优化、配置调整）时，不走完整版本流程，但必须：

1. 修改前读取 `Specs/technical/技术讲解.md` 了解相关模块；涉及接口时再读 `协议与数据.md`
2. 先写复现问题的失败测试（单元或集成），再修复
3. 修改后跑 `make check`；触及外部依赖时跑 `make test-integration`；接口行为有变化时再跑 `make smoke` 和 `api-verify` 对应接口
4. 更新 `技术讲解.md`（改动影响技术全景时）；涉及某版本功能时同步对应 `技术方案.md`；接口变化同步 Postman 集合
5. 涉及新增包、公共函数或成段新代码时，动手前先扫现有公共实现，遵循 DRY 红线

---

## 代码规范

### 包组织与依赖方向

- `cmd/{binary}/`：只做装配（配置、logger、依赖注入、server 生命周期），不写业务逻辑
- `internal/httpapi/`：路由注册、请求解析与校验、响应编码、HTTP 错误映射；不直接访问存储
- `internal/{domain}/`：业务规则、领域类型、存储端口接口（`type Repository interface`）；不 import `net/http` 和具体存储驱动
- `internal/{storage}/`（如 `internal/postgres/`）：实现领域包定义的端口接口；不 import `httpapi`
- 依赖方向只允许 `cmd -> httpapi -> domain <- storage`；出现反向 import 视为设计错误
- 不预置空目录，不为「将来可能需要」创建 Manager / Service / Factory 等无调用方的抽象；一个包只有一个类型时不单独拆文件

### 命名

- 包名小写单词，不用下划线和复数（`user` 而非 `users`、`userservice`）
- 导出类型名不重复包名（`user.Service` 而非 `user.UserService`）
- 接口按行为命名（`Repository`、`Notifier`），单方法接口用 `-er` 后缀
- 错误变量 `ErrXxx`，自定义错误类型 `XxxError`
- 测试函数 `TestXxx_场景`，表驱动用例 `name` 字段写清场景

### 错误处理

- 错误必须处理或显式向上返回，不得 `_ = err` 吞掉（写响应的 `Encode` 错误除外，已在 `writeJSON` 集中处理）
- 向上返回时用 `fmt.Errorf("动作: %w", err)` 包装并保留原错误，调用方用 `errors.Is` / `errors.As` 判断
- 领域层定义业务错误（如 `ErrNotFound`、`ErrConflict`），HTTP 层集中映射到状态码与 `error.code`，映射表见 `协议与数据.md`
- 未预期错误统一返回 500 `internal_error`，日志记录细节，响应不泄露内部信息
- 不用 `panic` 处理业务错误；`withRecover` 只兜底程序缺陷

### HTTP 层

- 路由用 `METHOD /path` 模式注册；路径参数用 `r.PathValue`
- 请求体解析：`json.NewDecoder(r.Body)` 并 `DisallowUnknownFields()`，限制体积（`http.MaxBytesReader`）
- 参数校验在 handler 或领域层完成，校验失败返回 400 `invalid_argument` 并说明字段
- 响应只通过 `writeJSON` / `writeError`，不手写 `w.Write`
- 需要请求级上下文（超时、取消、追踪）时通过 `r.Context()` 传递，不用全局变量

### 并发与资源

- 后台 goroutine 必须有退出路径（context 取消或 channel 关闭），启动处写明谁负责等待它结束
- 共享状态优先用 channel 或不可变数据，确需锁时锁的范围最小，且不在持锁时做 I/O
- 数据库连接池、HTTP client 等资源在 `cmd` 中创建并注入，在 `Shutdown` 时关闭
- 涉及并发的改动跑 `go test -race`

### 日志与配置

- 只用 `log/slog`，结构化键值；不用 `fmt.Println` / `log.Printf` 输出运行日志
- 日志不记录密码、token、完整请求体、个人敏感信息
- 配置只来自环境变量，在 `internal/config` 集中解析并给默认值；密钥不写入代码和文档

### 测试

- 三层分级，用构建标签隔离：
  - 单元测试：无 tag，文件名 `xxx_test.go`，`make test` 默认运行；不依赖网络、磁盘外部状态与真实服务
  - 集成测试：文件首行 `//go:build integration`，文件名 `xxx_integration_test.go`，`make test-integration` 运行；使用真实依赖（真实 TCP、数据库、缓存），连接信息来自环境变量，缺失时 `t.Skip` 并说明原因
  - 端到端（按需）：`//go:build e2e`，覆盖跨服务完整流程
- 测试文件与被测代码同包，表驱动，`t.Run` 分场景；每个测试自建 fixture（server、store、temp dir）并 `t.Cleanup`，不共享可变状态
- HTTP 层单元测试用 `httptest` 直接驱动 handler；集成测试用 `httptest.NewServer` 或真实二进制走网络
- 环境变量用 `t.Setenv`，临时文件用 `t.TempDir()`，不残留副作用
- 不写只为覆盖率的断言；每个测试至少验证一个可观察行为，每条需求验收标准至少对应一个测试
- deterministic 测试默认 `count=1`，只有并发 / 随机逻辑才用 `-race` 或 `-count` 重复
- 更多模式（race 常见坑、fuzz、契约与不变量测试）见 Skill `go-development` 的 `references/testing.md`、`fuzz-testing.md`、`contracts-and-invariants.md`

### DRY 红线

- 同一段逻辑（约 3 行以上）准备出现第二处时，先抽公共实现再写第二处；不要先复制再说
- 多个入口（单条 / 批量 / 定时）共用一个私有执行函数，差异用参数或闭包注入
- 枚举到状态码 / 错误码 / 文案的多处 `switch` 改成映射表
- 只有 1 处调用点或职责还看不清时先写在使用处，等第二处出现再上提；不为「将来可能复用」提前造公共件
- 抽象后需要 4 个以上参数或多个布尔开关，说明不该抽；在方案里写明理由保留重复
- 当前公共实现清单（新增后回填）：

| 需求 | 优先用 | 不要这么干 |
| --- | --- | --- |
| JSON 响应 / 错误响应 | `httpapi.writeJSON`、`httpapi.writeError` | handler 里手写 `w.Header().Set` + `json.Marshal` |
| 请求日志 / panic 兜底 | `httpapi.withRequestLog`、`httpapi.withRecover` | 各 handler 各自 `defer recover()` 或打印日志 |
| 环境变量配置 | `config.Load` | 业务包里直接 `os.Getenv` |

### 依赖使用

- 标准库优先；`net/http`、`log/slog`、`database/sql` 能满足时不引第三方框架
- 新增第三方依赖前告知用户：用途、替代方案、许可证、维护状态
- 引入后 `go mod tidy`，`go.sum` 入库

---

## 验证命令

| 命令 | 内容 | 何时跑 |
| --- | --- | --- |
| `make check` | `go vet` + 单元测试 + gofmt 检查 + `spec-check` | 每次代码改动后（必需） |
| `make spec-check` | 单独运行 Spec 一致性检查 | 改动 Specs 文档后 |
| `make spec-init VERSION=x.y.z` | 由模版生成该版本技术方案，要求人工需求已存在 | Step 4 |
| `go run ./cmd/spec-graph <init|status|record|finding|event|check> {version}` | 可选：版本生命周期状态图 | 启用 `spec-graph-workflow` 时 |
| `make test-integration` | `-tags integration` 集成测试 | 版本自验、触及外部依赖时（必需） |
| `make test-race` | 全仓竞态检测 | 并发相关改动（必需） |
| `make smoke` | 编译真实二进制、启动、curl 校验 | 接口改动后、版本自验（必需） |
| Skill `api-verify` | 按技术方案逐接口 curl 校验真实进程 | 版本自验、接口改动（必需） |
| `make lint` / `make vuln` | golangci-lint / govulncheck，未安装自动跳过 | 版本自验（可选） |
| `make run` | 启动服务（默认 `:8080`） | 用户验证 |
| Postman（MCP 或手动导入 `api/postman/`） | 运行集合断言 | 用户验证（可选工具） |

---

## Skills

| Skill | 用途 | 何时用 |
| --- | --- | --- |
| `init-project` | 从模板复制并初始化业务工程：重命名 module、填充项目信息、git 初始化 | 新建业务工程 |
| `spec-coding-init` | 将已有 Go 工程改造为 Spec Coding 模式：建 Specs 目录、同步 AI 资产、逆向生成技术讲解 | 存量项目接入 |
| `api-verify` | 启动服务，按技术方案 / Postman 集合逐接口 curl 验证并出报告 | Step 6.1、接口改动 |
| `sync-ai-assets` | 从模板工程同步 Skills、行为准则、Specs 模版、配置到当前工程 | 模板更新后 |
| `spec-graph-workflow`（可选） | 用 `spec-graph` 状态图 + Subagent（`spec-implementer`、`spec-reviewer`）展开一个版本：主会话作为唯一 Controller 记录证据、推进阶段 | 版本较大、需要独立审查与证据失效追踪时 |
| `go-development`（第三方） | Go 工程实践参考：测试分层、race 常见坑、架构、API 防御、slog、lint、fuzz、依赖升级 | Step 5 编码、Step 6 自验时按需加载 references |

`go-development` 来自 netresearch（MIT AND CC-BY-SA-4.0），来源、版本、适用范围与停用方法见 `.ai/skills/go-development/SOURCE.md`；它是参考资料，与本文件冲突时以本文件为准。

调用方式：Claude Code 用 `/{skill}`，Codex 用 `${skill}`。

---

## Subagent 与 Hook（Claude Code）

| 机制 | 位置 | 作用 | 边界 |
| --- | --- | --- | --- |
| Subagent `spec-reviewer` | `.claude/agents/spec-reviewer.md`，tools 仅 `Read, Grep, Glob` | 只读独立审查，返回 verdict 与 findings 的 YAML 交接合同 | 不修改文件、不运行命令；结论不替代 `make check` 等命令结果 |
| Subagent `spec-implementer` | `.claude/agents/spec-implementer.md` | 按技术方案切片测试先行实现，只跑 focused 测试，返回改动文件与 RED/GREEN 证据 | 不得改 `Specs/requirements/**`、`graph.json`、第三方 Skill；broad 门禁由主会话运行 |
| Hook `PostToolUse` | `.claude/settings.json` -> `scripts/check-format.sh` | `Edit` / `Write` 之后立即 gofmt 检查 `cmd/` 与 `internal/` | 事后反馈，不能撤销写入，不覆盖 Bash 改动，不是安全边界 |

Codex 无对应机制：审查在新会话中按 `spec-reviewer` 正文执行，格式检查依赖 `make check`。真正需要禁止的操作依靠权限、沙箱和人工确认，不依靠上述任一项。
