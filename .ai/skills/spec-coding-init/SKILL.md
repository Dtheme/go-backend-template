---
name: spec-coding-init
description: 将现有 Go 工程改造为 Spec Coding 模式——建立 Specs 目录、同步 AI 基础设施（行为准则、Skills、双 Agent 符号链接、配置）、扫描代码并逆向生成技术讲解，使后续开发遵循 AI Coding 工作流。同时支持 Claude Code 和 Codex。
---

# /spec-coding-init — Spec Coding 模式改造

> 一键将现有 Go 工程改造为 Spec Coding 驱动开发模式。
> 用法：`/spec-coding-init [项目目录路径] [--template 模板工程路径]`
> - 不传项目路径时使用当前工作目录
> - `--template` 指定 go-backend-template 所在路径；不传时按下方 Step 1.1 定位

## 交互规则

- **每次只问一个配置项**，等用户回复后再问下一个
- 用户输入 `跳过` 时保留占位或用代码分析结果推断，继续下一项
- 不提供默认值或建议值
- 改造过程不修改任何业务代码，只新增文档、AI 资产与符号链接

---

## 执行流程

### Step 0 — 确定 PROJECT_ROOT 与项目状态

**确定 PROJECT_ROOT**：传入路径或当前目录，必须包含 `go.mod`；不含则向上查找一级，仍没有则询问用户。

**判断是否已改造**：检查 `Specs/` 与 `.ai/ai-rules.md`

- 均存在：提示「该项目已启用 Spec Coding 模式」，询问是否需要补充技术讲解或同步资产（改用 `sync-ai-assets`）
- 部分存在：增量补齐缺失部分
- 均不存在：执行完整流程

**判断 AI 平台**（决定同步哪些配置）：

| 检测条件 | 平台 |
| --- | --- |
| 有 `.claude/` 或 `CLAUDE.md` | Claude Code |
| 有 `.codex/`、`.agents/` 或 `AGENTS.md` | Codex |
| 两者皆有 | 双 Agent |
| 两者皆无 | 默认启用双 Agent |

**禁止嵌套 `.git`**：`git status` 出现 `new file mode 160000` 说明子目录内有独立 `.git`，需 `git reset HEAD -- <path>` 后删除子目录 `.git` 再加入。

---

### Step 1 — 定位模板 + 建立 Specs 目录

#### 1.1 定位模板

```bash
TEMPLATE_ROOT="{--template 参数}"
# 未传参数时依次尝试：本 Skill 所在目录的 ../../..（Skill 随模板一起被复制时）；询问用户模板路径
test -f "$TEMPLATE_ROOT/go.mod" && grep -q 'go-backend-template' "$TEMPLATE_ROOT/go.mod" || { echo "模板路径无效"; exit 1; }
```

#### 1.2 建立 / 同步 Specs 目录

```
PROJECT_ROOT/Specs/
├── requirements/          # 人工维护，AI 只读
│   ├── 需求模版.md
│   └── 协议与数据.md
└── technical/             # AI 维护
    ├── 技术讲解.md
    └── 技术方案模版.md
```

| 文件 | 本项目已存在 | 本项目不存在 |
| --- | --- | --- |
| `requirements/需求模版.md` | 用模板版本覆盖 | 从模板复制 |
| `technical/技术方案模版.md` | 用模板版本覆盖 | 从模板复制 |
| `requirements/协议与数据.md` | **保留本项目内容** | 从模板复制，并在 Step 4 提示用户按实际协议修改 |
| `requirements/{version}/需求.md` | **保留** | 不创建（由用户按需创建） |
| `technical/技术讲解.md` | **保留**（Step 5 更新） | 先建空文件，Step 5 生成 |
| `technical/{version}/技术方案.md` | **保留** | 不创建（由 AI 按需创建） |

原则：模版文件以模板工程为准；业务内容以本项目为准。

---

### Step 2 — 同步 AI 基础设施

#### 2.1 迁移旧结构（如存在）

- `.claude/commands/{name}.md` → `.ai/skills/{name}/SKILL.md`（内容不改），删除 `.claude/commands/`
- `.claude/skills/` 是真实目录 → 内容移入 `.ai/skills/`，删除后建符号链接
- `CLAUDE.md` 是普通文件 → 内容移入 `.ai/ai-rules.md`（已存在则合并缺失章节），删除后建符号链接
- `AGENTS.md` 是普通文件 → 同上；`.ai/ai-rules.md` 已存在时只补充缺失章节

#### 2.2 从模板同步文件

**共享文件**（总是同步）：

| 模板路径 | 本项目路径 | 策略 |
| --- | --- | --- |
| `.ai/skills/*` | `.ai/skills/*` | 逐 Skill 目录：新增则复制，已有则对比内容并提示是否覆盖，本项目自有 Skill 保留 |
| `.gitignore` | `.gitignore` | 不存在则复制；已存在则确保包含 `bin/`、`.claude/settings.local.json` |
| `scripts/smoke.sh` | `scripts/smoke.sh` | 不存在则复制并提示按实际接口修改检查项；已存在则保留 |
| `scripts/spec-init.sh`、`scripts/check-format.sh` | 同路径 | 不存在则复制；已存在则以模板为准更新 |
| `cmd/spec-check/`、`internal/speccheck/`、`internal/specdoc/` | 同路径 | 必须复制（`make check` 依赖）；已存在则以模板为准更新。复制后 import 路径要改为本项目 module |
| `cmd/spec-graph/`、`internal/specgraph/`、`docs/spec-graph/`、`.ai/skills/spec-graph-workflow/` | 同路径 | 可选扩展：询问用户是否启用；启用才复制并改 import 路径 |
| `api/postman/` | `api/postman/` | 不存在则创建目录并生成以项目命名的空集合（含 healthz 请求）；已存在则保留 |

**Claude Code 平台文件**（支持 Claude Code 或默认双 Agent 时）：

| 模板路径 | 本项目路径 | 策略 |
| --- | --- | --- |
| `.claude/settings.json` | `.claude/settings.json` | 不存在则复制；已存在则合并 `permissions.allow` 与 `hooks.PostToolUse`（格式检查 Hook） |
| `.claude/agents/*.md` | `.claude/agents/` | 不存在则复制 `spec-reviewer.md`、`spec-implementer.md`；已存在则保留并提示差异 |

**Codex 平台文件**（支持 Codex 或默认双 Agent 时）：

| 模板路径 | 本项目路径 | 策略 |
| --- | --- | --- |
| `.codex/config.toml` | `.codex/config.toml` | 不存在则复制；已存在则保留 |

#### 2.3 确保符号链接

| 符号链接 | 指向 | 平台 |
| --- | --- | --- |
| `.claude/skills` | `../.ai/skills` | Claude Code |
| `.agents/skills` | `../.ai/skills` | Codex |
| `CLAUDE.md` | `.ai/ai-rules.md` | Claude Code |
| `AGENTS.md` | `.ai/ai-rules.md` | Codex |

不存在或断裂则创建 / 重建（先 `mkdir -p` 父目录）。

#### 2.4 Makefile

- 不存在：从模板复制
- 已存在：确认有 `check`（vet + test + gofmt，且依赖 `spec-check`）、`spec-check`（`go run ./cmd/spec-check`）、`spec-init`（`sh scripts/spec-init.sh $(VERSION)`）、`test-integration`（`-tags integration`）、`test-race`、`smoke` 六个目标；缺失则追加并说明

---

### Step 3 — 分析现有代码

> 扫描依据是项目实际结构，不是 Skill 里写死的路径。先发现全貌，再按需深入。

#### Stage A — 结构发现

1. `go.mod`：module 路径、Go 版本、直接依赖（`require` 非 `// indirect`）
2. `go list ./...`：全部包清单；统计 `.go` 文件数与 `_test.go` 数
3. 入口：`cmd/*/main.go` 或根目录 `main.go`；读取 `main` 函数，追踪初始化顺序（配置、日志、连接、路由、server）
4. 路由：搜索 `HandleFunc(`、`Handle(`、`.GET(`、`.POST(`、`Route(`、`Group(`（覆盖 net/http、gin、echo、chi、fiber 等），提取完整路由表
5. 中间件：搜索 `func(next http.Handler)`、`.Use(`，列出链路顺序
6. 配置：搜索 `os.Getenv`、`envconfig`、`viper`、`flag.`，列出配置项与默认值
7. 存储：搜索 `database/sql`、`pgx`、`gorm`、`sqlx`、`redis`、`mongo`；找迁移目录（`migrations/`、`db/`）与工具（golang-migrate、goose、atlas）
8. 外部调用：搜索 `http.Client`、`grpc.Dial`、消息队列客户端
9. 质量基线：`go vet ./...`、`go test ./... -cover` 结果（只记录，不修复）

#### Stage B — 模块深入（按 Stage A 结果动态选择）

| 目录名模式 | 模块类型 | 分析重点 |
| --- | --- | --- |
| `cmd/` | 入口 | 每个二进制的职责、装配顺序、优雅停机 |
| `internal/httpapi/`、`handler/`、`api/`、`transport/` | HTTP 层 | 路由表、请求解析、响应格式、错误映射 |
| `internal/{domain}/`、`service/`、`domain/`、`usecase/` | 业务层 | 核心类型、业务规则、端口接口 |
| `internal/{storage}/`、`repository/`、`store/`、`db/` | 存储层 | 实现了哪些端口、事务边界、SQL 组织方式 |
| `pkg/` | 可复用库 | 对外导出的能力 |
| `migrations/` | 数据变更 | 迁移顺序、最新 schema |
| `config/`、`internal/config/` | 配置 | 配置项、来源、校验 |
| `middleware/` | 中间件 | 鉴权、日志、限流、恢复 |
| `scripts/`、`Makefile`、`.github/` | 工程化 | 可用命令、CI 门禁 |

每个模块：列出文件、读取导出类型与函数、追踪 import 建立依赖图、标记与「代码规范 / 依赖方向」冲突之处。

**废弃代码识别**：`go vet` 未引用警告、无调用方的导出函数（`grep -rn` 确认）、注释掉的大段代码、`_test.go` 缺失的核心包；记入技术讲解「已知问题」。

---

### Step 4 — 收集项目信息

逐一询问（`跳过` 则用代码分析结果推断）：

| 询问项 | 更新目标 |
| --- | --- |
| 服务名称 | `ai-rules.md`、`技术讲解.md` |
| 一句话描述 | `ai-rules.md`、`技术讲解.md` |
| 架构模式（如分层 / 六边形 / 单包） | `技术讲解.md` |
| 主要技术栈（框架、数据库、消息队列） | `技术讲解.md` |
| 接口验证方式（Postman / curl / 其他） | `ai-rules.md` 验证命令 |

同时提示用户：`Specs/requirements/协议与数据.md` 需按项目实际协议修改（AI 不改该文件）。

---

### Step 5 — 生成 AI 行为准则

创建或增量更新 `.ai/ai-rules.md`，以模板 `.ai/ai-rules.md` 为基底，按项目实际调整：

1. **项目简介**：Step 4 信息
2. **目录结构**：Step 3 实际结构映射到 Spec Coding 规范结构
3. **文件权限规则**：保留标准规则
4. **能力前置校验**：保留；项目依赖数据库等时追加对应探针
5. **AI 工作流**：完整 Step 1-7，不得简化；项目无 Postman 时把 6.2 的验证方式改为用户指定方式
6. **日常代码修改**：保留
7. **代码规范**：按项目实际调整包组织、命名、路由框架（gin / chi 等）的写法，保留错误处理、日志、测试、DRY 红线；「当前公共实现清单」按 Step 3 发现的公共函数填写
8. **验证命令**、**Skills**、**Subagent 与 Hook**：按项目实际 Makefile 目标、已同步 Skills 与启用的平台填写；只支持 Codex 的项目删除 Hook / Subagent 行并说明替代方式

已存在 `.ai/ai-rules.md` 时：新增章节插入，已有章节保留项目特有内容只补充模板增强的规则，**禁止直接覆盖**。

---

### Step 6 — 逆向生成技术讲解

**读取顺序**：

第一轮（核心骨架）：入口 `main`、配置、路由表、中间件链、存储初始化、`go.mod`
第二轮（业务模块）：逐个领域包、存储实现、迁移文件、主要类型
第三轮（补充）：工具包、脚本、CI、外部调用、鉴权、限流、缓存

**生成 `Specs/technical/技术讲解.md`**，章节：

```markdown
# 技术讲解

## 项目概述
- 服务名称、描述、架构、Go 版本、主要依赖、对外接口概览

## 目录结构
<!-- 树形结构，每个目录 / 关键文件一行说明 -->

## 核心模块
### 入口与生命周期
### 配置
### HTTP 层（路由表、中间件顺序、响应与错误格式）
### 业务层（按领域分节：职责、核心类型、业务规则、端口接口）
### 存储层（实现、事务边界、迁移方式）
### 外部依赖调用
### 鉴权 / 限流 / 缓存（如有）

## 接口清单
<!-- 方法 | 路径 | 说明 | 鉴权 | 响应 -->

## 数据模型
<!-- 表 / 集合、关键字段、唯一约束、索引 -->

## 测试策略
<!-- 现有测试分布、覆盖率、集成测试开关、验证命令 -->

## 第三方依赖
<!-- go.mod 直接依赖：包 | 版本 | 用途 -->

## 关键设计约定
<!-- 约定 | 位置 -->

## 已知问题与待优化项
<!-- 废弃代码、依赖方向违规、缺测试的核心包、vet 警告、性能隐患 -->
```

**生成策略**：先读核心骨架建立全局认知，并行读取业务模块，生成后对比 `go list ./...` 确认每个包都有覆盖。

---

### Step 7 — 收尾

1. **验证结构**：`Specs/` 四个文件、`.ai/ai-rules.md`、对应平台的符号链接均存在且指向正确
2. **验证基础设施**：
   - `.ai/skills/` 含 `init-project`、`spec-coding-init`、`api-verify`、`sync-ai-assets`、`go-development`（含 `SOURCE.md` 与许可证）
   - 项目已有测试按三层分级：无 tag 单元测试可由 `make test` 运行；依赖真实外部服务的测试已加 `//go:build integration`，否则在报告中列为待整理项（不自动改动）
   - `.gitignore` 含 `bin/`、`.claude/settings.local.json`
   - `make spec-check` 通过：项目已有 `Specs/{requirements,technical}/{version}/` 目录时，需求文件的功能编号必须是 `F-{version}-NNN`、技术方案含「交付状态」块；不满足的列为待整理项并说明改法（AI 不改人工需求）
   - Claude Code：`.claude/settings.json`（含 Hook）、`.claude/agents/`、`.claude/skills` 链接、`CLAUDE.md` 链接
   - Codex：`.codex/config.toml`、`.agents/skills` 链接、`AGENTS.md` 链接
3. **验证不破坏构建**：`make check`（或 `go build ./... && go test ./...`）与改造前结果一致
4. **输出改造报告**：创建的文件、同步的基础设施（按平台）、技术讲解模块覆盖情况、废弃代码与问题清单、启用的 AI 平台
5. **提示用户**：
   - 后续流程：在 `Specs/requirements/{version}/需求.md` 写需求 → AI 出技术方案 → 确认 → 编码 → 验证 → 更新文档
   - review 技术讲解与 ai-rules.md，补充 AI 未能分析的部分
   - 按实际协议修改 `协议与数据.md`
   - 清理已识别的废弃代码
