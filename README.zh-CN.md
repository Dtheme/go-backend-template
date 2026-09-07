[English](README.md) | 简体中文

# go-backend-template

一个只依赖标准库的最小 Go HTTP 服务，自带面向编码 Agent 的 Spec 工作流。Claude Code 和 Codex 读同一份行为准则，`make check` 会机器核对文档是否还描述着当前代码。

## 快速开始

```bash
make run     # :8080
make check   # go vet + 单元测试 + gofmt + spec-check
```

从模板复制出业务工程，脚本会替换 module 路径、重建符号链接，并验证副本能编译能跑测试：

```bash
bash .ai/skills/init-project/rename_module.sh my-service github.com/me/my-service
```

Agent 可以直接跑同一件事：Claude Code 用 `/init-project`，Codex 用 `$init-project`。想给已有 Go 工程补上这套工作流，用 `spec-coding-init`。

## 命令

| 命令 | 内容 |
| --- | --- |
| `make check` | `go vet`、单元测试、`gofmt -l`、`spec-check` |
| `make test-integration` | 带 `//go:build integration` 标签的测试 |
| `make test-race` | `go test -race ./...` |
| `make smoke` | 编译真实二进制、启动、逐条 curl |
| `make spec-init VERSION=x.y.z` | 生成该版本技术方案，人工需求不存在就拒绝 |
| `make lint` / `make vuln` | golangci-lint / govulncheck，未安装自动跳过 |

`spec-check`（`cmd/spec-check`）用十条规则约束文档：必需文件与章节、四条符号链接、版本目录命名、需求里每个 `F-x.y.z-NNN` 都要出现在技术方案里、交付状态块、Postman 集合的结构。

## 一个版本怎样做出来

`Specs/requirements/` 归人维护，Agent 只读。`Specs/technical/` 归 Agent 维护，必须跟着代码走。

1. 读技术讲解、本版本需求、协议与数据约定。
2. 有歧义就问，不允许猜。
3. 冻结 API 契约：方法、路径、请求、响应、错误表。
4. `make spec-init VERSION=x.y.z`，填完方案，**等用户确认**，把 `stage` 改成 `implementing`。
5. 先写失败测试，再写让它通过的最小实现。
6. 跑四道门禁加 `api-verify` Skill，然后**等用户验收**。
7. 把真实结果回写进技术方案、技术讲解、Postman 集合和冒烟脚本。

两处必须停下来等人。Step 4 不确认就不能动代码，Step 6 不验收就不算交付。`user_acceptance` 还是 `pending` 时 `spec-check` 会拒绝 `stage: delivered`，这道停顿由命令保证，不靠自觉。

## 仓库里有什么

| 机制 | 位置 |
| --- | --- |
| 行为准则单源 | `.ai/ai-rules.md`，符号链接为 `CLAUDE.md` 和 `AGENTS.md` |
| Spec 文档 | `Specs/requirements/`（人工）、`Specs/technical/`（Agent） |
| 门禁 | `Makefile`、`cmd/spec-check`、`scripts/smoke.sh` |
| Skill | `.ai/skills/`：`init-project`、`spec-coding-init`、`api-verify`、`sync-ai-assets`、`spec-graph-workflow`，外加内置的 [`go-development`](https://github.com/netresearch/go-development-skill) |
| Subagent | `.claude/agents/`：`spec-reviewer`（只读）、`spec-implementer` |
| Hook | `PostToolUse` → `scripts/check-format.sh`，每次编辑后做 gofmt 反馈 |
| MCP | 不需要任何 MCP；Postman MCP 可选，且从不参与门禁 |

### 内置的第三方 Skill

`.ai/skills/go-development/` 不是这里写的，它原样复制自 [netresearch/go-development-skill](https://github.com/netresearch/go-development-skill)，固定在 1.15.1 版本，补的是本模板不重复讲的 Go 实践：测试分层、`-race` 常见坑、`slog`、lint、fuzz、依赖升级。

该目录的 `SOURCE.md` 就是引用记录：上游地址、固定版本、26 个文件的 SHA-256（任何人都能重算核对有没有被改动）、许可证拆分，以及上游建议与 `.ai/ai-rules.md` 冲突的五处。这五处一律以本地规则为准。升级的做法是整目录替换并重写 `SOURCE.md`，不要就地打补丁，否则校验和就失去意义。

## 演示版本

`1.0.0` 用内存存储实现两个接口，让整套工作流有真实对象可跑：

```
POST /v1/notes       201 {"id","title","content","created_at"}
GET  /v1/notes/{id}  200，或 404 {"error":{"code":"not_found","message":"note not found"}}
```

它的需求、技术方案、测试、六条审查 finding 和状态图都在仓库里。开始自己的项目时删掉 `Specs/*/1.0.0/`、`internal/note/`、`internal/memstore/` 和 `internal/httpapi/notes*.go`。

## 可选：spec-graph

`cmd/spec-graph` 记录每份证据属于哪个代码候选，门禁缺失或输入变过就拒绝推进版本。六个阶段、七类事件，每条转换都有守卫。说明见 `docs/spec-graph/`。

## 边界

这是一个 MVP。这里的规则、Skill、Subagent 和 Hook 只覆盖一个通用 Go HTTP 服务，再多没有。真实项目需要自己补：部署流程的 Skill、数据库和内部服务的 MCP、自己 CI 的门禁、自己业务域的审查规则。仓库里的东西是起点形状，不是成品工具链。

## 文档

18 篇详解：[docs/guide/](docs/guide/)，在线阅读版见 [飞书文档](https://ncnhrkchbfwf.feishu.cn/wiki/SVHxwROhfiYBE2k2I35cJ0IUnYe)。

## 许可证

MIT，`.ai/skills/go-development/` 除外。该目录内置自 [netresearch/go-development-skill](https://github.com/netresearch/go-development-skill)：代码与脚本是 MIT，20 篇 references 文档是 CC-BY-SA-4.0，著作权归 Netresearch DTT GmbH。改动这些文档后仍须保持 CC-BY-SA-4.0 并署名。固定版本与逐文件校验和在它的 `SOURCE.md` 里。不想承担 share-alike 义务，删掉这个目录即可。
