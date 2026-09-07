---
title: Skill 落地：五个自研 Skill 分别接管生命周期的哪个时刻
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - skills
  - init-project
  - spec-coding-init
  - sync-ai-assets
  - api-verify
---

# Skill 落地：五个自研 Skill 分别接管生命周期的哪个时刻

`.ai/ai-rules.md` 已经写了七个 Step、文件权限和验证命令（见「Rules 落地」和「版本开发工作流」），Skill 不重复这些。Skill 管的是生命周期里步骤多、跨文件、容易漏项的五个时刻：从模板新建工程、存量工程接入、模板更新后同步、Step 6.1 对真实进程的契约核对，还有可选的版本编排。每个 Skill 就是一份带停止条件的操作步骤。它自己不运行命令，也不限制工具，门禁还是 `make check` 那几条命令（见「Command 落地」）。

## Skill 放在哪里，怎样被两个 Agent 同时看到

所有 Skill 都在 `.ai/skills/{name}/SKILL.md`。frontmatter 只有 `name` 与 `description` 两个字段，正文从 `# /{name} — 用途` 开始，往下是用法说明和按 Step 编号的执行流程。要向用户提问的三个 Skill 多一节「交互规则」。`.claude/skills` 与 `.agents/skills` 是两条指向 `../.ai/skills` 的符号链接，Claude Code 敲 `/{name}`、Codex 敲 `${name}`，读到的是同一份文件。`CLAUDE.md`、`AGENTS.md` 一起链到 `.ai/ai-rules.md`，用的是同一招。

`init-project` 和 `spec-coding-init` 共用一组交互规则。每次只问一个配置项，等用户回答完再问下一个；用户回 `跳过` 就保留占位，`spec-coding-init` 可以改用代码分析结果推断；不给默认值，也不给建议值；`init-project` 首次运行的必填项不接受跳过。`sync-ai-assets` 的规则另写：先展示变更摘要，等用户确认才动手，冲突项一条条展示差异再问，本项目自有的 Skill、章节、配置全部保留。

| Skill | 接管的时刻 | 入口 |
| --- | --- | --- |
| `init-project` | 模板目录变成业务工程 | `/init-project`，内部调用 `rename_module.sh` |
| `spec-coding-init` | 已有 Go 工程接入 Spec 工作流 | `/spec-coding-init [项目目录路径] [--template 模板工程路径]` |
| `sync-ai-assets` | 模板更新后回流到业务工程 | `/sync-ai-assets [模板工程路径]` |
| `api-verify` | Step 6.1 自验的最后一道核对 | `/api-verify [version]` |
| `spec-graph-workflow` | 可选：版本较大、需要独立审查与证据失效追踪 | `/spec-graph-workflow {version}` |

`.ai/skills/` 下还有第六个目录 `go-development`，第三方 Skill，目录里带 `SOURCE.md`，AI 禁止修改，见「第三方 Skill」。

## init-project：模板变成业务工程的那一刻

Step 0 读当前目录的 `go.mod`。module 还是 `example.com/go-backend-template`，说明人还在模板里，走 Step 1 复制加重命名；换成了别的 module，说明已经是业务工程，Step 1 跳过；没有 `go.mod` 就提示并停下。接着扫五处占位符：`.ai/ai-rules.md` 项目简介、`技术讲解.md` 项目概述、`README.md` 标题、Postman 集合的 `info.name`、`CLAUDE.md` / `AGENTS.md` 符号链接是否断裂。扫描结果决定这次是首次运行，还是只补未配置项的后续运行。

Step 1 逐一询问新目录名与 Go module 路径，然后交给脚本：

```bash
# .ai/skills/init-project/SKILL.md，Step 1
TEMPLATE_ROOT="$(pwd)"
bash "$TEMPLATE_ROOT/.ai/skills/init-project/rename_module.sh" "$NEW_DIR_NAME" "$NEW_MODULE" "$(dirname "$TEMPLATE_ROOT")"
```

`rename_module.sh` 用 `set -euo pipefail` 跑。动手前先校验四件事：目录名只由字母、数字、下划线、连字符组成且不以数字开头，module 路径不等于模板值也不含非法字符，模板 `go.mod` 的 module 行确实是模板值，目标目录不存在。都过了才分五步执行：

| 步骤 | 做什么 | 边界 |
| --- | --- | --- |
| `[1/5]` 复制模板 | `tar -cf - . \| tar -xf -` 管道复制到目标父目录（默认模板同级） | 排除 `.git`、`bin`、`.claude/settings.local.json`、`.DS_Store`；tar 保留符号链接，也保留存于扩展属性的 macOS 红色标签 |
| `[2/5]` 替换 module 路径 | 对 `*.go`、`go.mod`、`*.md`、`*.sh`、`*.json`、`Makefile` 中含旧 module 的文件做 `sed` | 跳过 `.ai/skills/init-project/`，脚本自身不被改写 |
| `[3/5]` 替换模板名称 | 对 `*.md`、`*.json`、`Makefile`、`smoke.sh` 替换 `go-backend-template` | 跳过整个 `.ai/skills/`；Postman 集合文件按新名重命名 |
| `[4/5]` 检查符号链接 | `relink` 处理 `CLAUDE.md`、`AGENTS.md`、`.claude/skills`、`.agents/skills` | 不是符号链接或已断裂时才重建 |
| `[5/5]` 验证 | `go build ./... && go test ./...` | 没有 `go` 命令时提示手动执行 `make check` |

第二、三步的替换靠一个跨平台的 `sedi`。BSD sed 与 GNU sed 的 `-i` 语法不一样：

```bash
# .ai/skills/init-project/rename_module.sh
if sed --version >/dev/null 2>&1; then
    sedi() { sed -i "$@"; }
else
    sedi() { sed -i '' "$@"; }
fi
```

脚本跑完输出工程路径和修改文件清单，Skill 把路径记成 `PROJECT_ROOT`，后面所有操作都用这个绝对路径。Step 2 收服务名称和一句话描述，这两项必填；默认监听端口、主要外部依赖、鉴权方式可选。Step 3 只用 Edit 精确替换用户给了值的项，端口改动要同时改 `internal/config/config.go` 和 `config_test.go`，动了 Go 文件就跑 `make check`。Step 4 在没有 `.git` 时执行 `git init` 和首次提交，逐项确认 `.ai/ai-rules.md`、两组符号链接、`.claude/settings.json`、两个 Subagent 文件和 `.codex/config.toml` 都在，期望 `spec-check` 输出 `spec-check ok (0 version(s))`。这个 0 有前提。复制会把模板自带的 1.0.0 演示版本目录一起带走，`Specs/requirements/1.0.0/需求.md` 首段写了业务工程可以删掉它；不删，版本计数就不是 0。

## spec-coding-init：存量工程接入的那一刻

起点不同。`init-project` 从模板出发，`spec-coding-init` 面对的是一个已经在跑的 Go 工程，于是多一条交互规则：改造过程不碰任何业务代码，只新增文档、AI 资产与符号链接。

Step 0 先定 `PROJECT_ROOT`，目录必须含 `go.mod`，没有就向上找一级，还没有就问用户。看 `Specs/` 与 `.ai/ai-rules.md` 在不在，判断已改造、部分存在还是未改造；看 `.claude/`、`CLAUDE.md`、`.codex/`、`.agents/`、`AGENTS.md`，判断启用 Claude Code、Codex 还是双 Agent，两边都没有就默认双 Agent。嵌套 `.git` 明确禁止。Step 1 定位模板，建 `Specs/` 下四个文件：`需求模版.md` 与 `技术方案模版.md` 以模板为准覆盖，`协议与数据.md` 和已有版本目录留本项目的，`技术讲解.md` 已有就留、没有先建空文件，内容等 Step 6 逆向生成。

Step 2 同步 AI 基础设施。先迁旧结构：`.claude/commands/{name}.md` 变成 `.ai/skills/{name}/SKILL.md`，真实目录形态的 `.claude/skills` 和普通文件形态的 `CLAUDE.md`、`AGENTS.md` 都收进 `.ai/` 再改成链接。然后从模板同步文件。`cmd/spec-check/`、`internal/speccheck/`、`internal/specdoc/` 必须复制，`make check` 依赖 `spec-check`，复制完 import 路径改成本项目 module。`cmd/spec-graph/`、`internal/specgraph/`、`docs/spec-graph/`、`.ai/skills/spec-graph-workflow/` 是可选扩展，问过用户才复制。Makefile 要确认 `check`、`spec-check`、`spec-init`、`test-integration`、`test-race`、`smoke` 六个目标都在。

Step 3 是两阶段扫描，依据是项目实际结构，不是 Skill 里写死的路径。Stage A 结构发现固定九项：

1. `go.mod` 的 module 路径、Go 版本与直接依赖
2. `go list ./...` 的包清单，以及 `.go` 与 `_test.go` 文件数
3. 入口 `main` 函数的初始化顺序
4. 路由注册调用，覆盖 `net/http`、gin、echo、chi、fiber 的写法
5. 中间件链路顺序
6. 配置项与默认值
7. 存储驱动、迁移目录与迁移工具
8. 外部调用（`http.Client`、`grpc.Dial`、消息队列客户端）
9. `go vet ./...` 与 `go test ./... -cover` 的质量基线，只记录不修复

Stage B 拿 Stage A 的结果动态挑要深入的模块：

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

每个模块追 import 建依赖图，标出与「代码规范 / 依赖方向」冲突的地方。废弃代码记进技术讲解的「已知问题」，包括 `go vet` 警告、没有调用方的导出函数、注释掉的大段代码、缺测试的核心包。Step 4 收服务名称、描述、架构模式、技术栈、接口验证方式。Step 5 以模板 `ai-rules.md` 为基底生成行为准则，AI 工作流的 Step 1-7 不许简化，已有文件只补章节，禁止直接覆盖。Step 6 按三轮读取顺序逆向生成 `技术讲解.md`，生成后跟 `go list ./...` 对一遍，确认每个包都写到了。Step 7 验证结构、`make spec-check` 通过、`make check` 与改造前一致，再输出改造报告。

## sync-ai-assets：模板更新后的那一刻

模板会继续改，业务工程不能靠手工比对回流。Step 1 接受本地目录或 git 地址，后者 `git clone --depth 1` 到 `mktemp -d` 目录，再用 `go.mod` 里有没有 `go-backend-template` 确认它真的是模板。Step 2 分六类对比：

| 类别 | 对比方式 | 保留或交由用户决定的部分 |
| --- | --- | --- |
| 2.1 Skills | 逐 Skill 目录内所有文件，分为新增、更新、无变化、仅本地 | 仅本地 Skill 不动；第三方 Skill 按 `SOURCE.md` 版本号比较，模板更新时整目录替换，不逐文件合并 |
| 2.2 行为准则 | 按 `##` 章节拆分，新增章节插入，内容不同的展示差异 | 项目简介、目录结构、当前公共实现清单默认保留本项目内容 |
| 2.3 Specs 模版 | `需求模版.md`、`技术方案模版.md` 直接覆盖 | `协议与数据.md` 不同步 |
| 2.4 配置与脚本 | `settings.json` 合并 `permissions.allow` 与 `hooks.PostToolUse`；`check-format.sh`、`spec-init.sh` 以模板为准；Makefile 对比六个目标 | `spec-reviewer` 的 `tools` 必须保持 `Read, Grep, Glob`；`smoke.sh`、`.codex/config.toml` 展示差异由用户决定 |
| 2.5 符号链接 | 检查四条链接 | 断裂或缺失记为「需修复」 |
| 2.6 工具包 | `cmd/spec-check/`、`internal/speccheck/`、`internal/specdoc/` 缺失则新增、不同则更新；spec-graph 四项可选 | 整目录复制覆盖，因为这些目录不含业务代码 |

Step 3 用固定格式输出变更摘要，Step 4 等用户确认，Step 5 才执行。Step 6 删掉临时 clone 目录，跑 `make check`，结果与同步前一致才算完；本项目已有 `graph.json` 的，还要跑 `go run ./cmd/spec-graph check {version}`。第三方 Skill 整目录替换，是「AI 禁止修改第三方 Skill」这条权限规则落在同步时刻的样子：`go-development` 当前 1.15.1，26 个上游文件的 SHA-256 记在 `SOURCE.md`，本地改动为零。

## api-verify：自验最后一道核对

`api-verify` 接管 Step 6.1 的第 5 项。先确认 `make check` 与 `make test-integration` 已经通过，再在 `mktemp -d` 目录编译真实二进制，用 `HTTP_ADDR=":18090"` 启动，端口与 `scripts/smoke.sh` 的 18080 错开。契约里每个接口至少核对成功路径、每种错误场景、方法不匹配的 405，以及契约声明的幂等行为。然后核对 Postman 集合，还有每个场景对应的测试用例名。收尾是 `kill` 进程、删运行目录、`lsof` 确认端口释放。1.0.0 的实录：17 项场景全部通过，服务器日志不含笔记正文，Postman 集合 7 个请求，没有接入 Postman MCP 所以未使用。手动 curl 通过、但没有对应测试的行为不算已验证。自验通过之后还要等用户确认，这条边界展开见「验证闭环」。

## spec-graph-workflow：可选的版本编排

`spec-graph-workflow` 不动七个 Step，只把它们编排成 `planning`、`implementing`、`reviewing`、`fixing`、`verifying`、`ready_to_deliver` 这张确定性状态图。主会话是唯一 Controller，负责跑门禁，用 `cmd/spec-graph` 的 `record`、`finding`、`event` 记证据、推阶段；实现委派给 `spec-implementer`，审查委派给 `spec-reviewer`。用户决定只写在技术方案的「交付状态」块里，Graph 读它当守卫输入。1.0.0 的 `graph.json` 走到 revision 46、stage `verifying`，最终门禁记录挂在候选 `2d03ec4f1c56` 上，六条 finding 全部 verified。`event 1.0.0 verified` 还是被守卫用退出码 3 拒了，理由只有一条：`user_acceptance` 是 `pending`，要 `confirmed`。Loop 管局部收敛，Graph 管结论是否仍然有效；CLI、`graph.json` 与编排细节见「Spec + Graph 理论」和「Spec + Graph 落地」。

## Skill、Subagent、Hook 各管什么

三种机制都在仓库里，触发方式和能力边界各不相同：

| 机制 | 位置 | 触发 | 能做 | 不能做 |
| --- | --- | --- | --- | --- |
| Skill | `.ai/skills/{name}/SKILL.md` | 用户 `/name`、`$name`，或 `ai-rules.md` 指定的 Step | 给出步骤、停止条件与报告格式 | 不限制工具，不自动运行，不替代 `make` 门禁 |
| Subagent `spec-reviewer` | `.claude/agents/spec-reviewer.md`，tools `Read, Grep, Glob` | 主会话在 6.1 通过后委派 | 只读三轮审查，返回 verdict 与 findings | 不改文件、不跑命令；结论不替代命令结果 |
| Subagent `spec-implementer` | `.claude/agents/spec-implementer.md`，tools `Read, Grep, Glob, Edit, Write, Bash` | `spec-graph-workflow` 按切片委派 | 测试先行实现，只跑 focused 测试 | 不跑 broad 门禁，不改 `Specs/requirements/**`、`graph.json`、第三方 Skill |
| Hook `PostToolUse` | `.claude/settings.json`，matcher `Edit\|Write` | 每次 Edit / Write 之后 | 运行 `scripts/check-format.sh`，未格式化时输出 `gofmt needed:` 并以退出码 2 结束 | 事后反馈，不能撤销写入，不覆盖 Bash 改动 |

1.0.0 的实录记下了这些边界在真实会话里的形态。`spec-implementer` 是通用 Agent 加载 `.claude/agents/spec-implementer.md` 正文扮演的角色，不是 Claude Code 原生 agents 目录加载；它返回 11 个改动文件和每个用例的 RED/GREEN 记录，broad 门禁由主会话跑。`spec-reviewer` 第一轮 `changes_required`，给出 5 条 finding，第二轮 `pass`，追加 1 条 P3。`check-format.sh` 用未格式化探针验过退出码 2，删掉探针后退出码 0；PostToolUse 在真实 Claude Code 会话里的自动触发还没有记录。Codex 那边没有 Hook 和 Subagent，格式检查靠 `make check`，审查另开会话按 `spec-reviewer` 正文执行。三者的完整边界见「Subagent 与 Hook 落地」。

Skill 接管的是时刻，不是规则。规则在 `ai-rules.md` 一处维护，门禁在 `make` 目标与 `spec-check` 里执行。Skill 只保证新建、接入、同步、自验、编排这五个容易漏项的节点上，两个 Agent 走的是同一套步骤，并在同一个位置停下来等用户。
