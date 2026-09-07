---
title: Command 落地：make 目标、脚本与 spec-check 各自守什么
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - command
  - makefile
  - spec-check
  - smoke
---

# Command 落地：make 目标、脚本与 spec-check 各自守什么

在 go-backend-template 里说「跑一条命令」，可能是让 make 执行一个目标，也可能是让 Agent 按某个 Skill 的步骤走一遍。旁边还有第三种东西经常被算进来：委派 Subagent 得到的审查结论。三者最后都会写出「通过」两个字。只有第一种带退出码，下一个人能原样重跑。

## 三种「通过」的分量并不相同

| 形式 | 例子 | 执行者 | 结果形态 | 能证明什么 |
|---|---|---|---|---|
| Makefile 目标与 `scripts/` | `make check`、`make smoke`、`scripts/spec-init.sh` | shell 与 Go 工具链 | 退出码加标准输出 | 某份代码在某台机器上通过了某项确定性检查 |
| 斜杠命令 | `/api-verify`、`/init-project`、`/spec-graph-workflow` | Agent 读取 `.ai/skills/<name>/SKILL.md` 后逐步执行 | Agent 写出的报告 | 步骤被照做时得到的观察 |
| Subagent | `spec-reviewer`、`spec-implementer` | 主会话委派的独立上下文 | YAML 交接合同 | 一次独立阅读或一段实现，不是命令结果 |

`.ai/ai-rules.md` 的「Skills」一节写明调用方式：Claude Code 用 `/{skill}`，Codex 用 `${skill}`。两个名字指向 `.ai/skills/` 下同一份 SKILL.md，`.claude/skills` 与 `.agents/skills` 都是指向 `../.ai/skills` 的符号链接。斜杠命令的确定性来自 SKILL.md 写得可不可复现，跟 shell 无关。`api-verify` 的 SKILL.md 自己就是这么定位的：前置条件是 `make check` 与 `make test-integration` 已通过，它只确认真实二进制与契约一致，不替代测试。

Subagent 更不是命令。`spec-reviewer` 的工具只有 `Read, Grep, Glob`，规则表写明「结论不替代 `make check` 等命令结果」。`spec-implementer` 能跑 `go test ./internal/<pkg>/...` 这类 focused 测试，`make check`、`make smoke` 与 `api-verify` 则由主会话作为唯一执行者运行。这与 Skill / Subagent / Hook 分工里「Subagent 结论不替代命令」是同一条边界，「Subagent 与 Hook 落地」展开。

## Makefile 的十四个目标各守什么

`.PHONY` 一行列出全部 14 个目标：

| 目标 | 实际命令 | 守什么 |
|---|---|---|
| `run` | `go run ./cmd/api` | 用户验证时启动服务，默认 `:8080` |
| `build` / `clean` | `go build -o bin/api ./cmd/api` / `rm -rf bin` | 编译产物与清理 |
| `test` | `go test ./...` | 无 tag 的单元测试 |
| `test-integration` | `go test -tags integration ./...` | 带 `//go:build integration` 的真实链路测试 |
| `test-race` | `go test -race ./...` | 全仓竞态检测 |
| `fmt` / `vet` | `gofmt -w .` / `go vet ./...` | 格式化写回；静态检查 |
| `check` | 前置 `vet test spec-check`，再 `gofmt -l .` | 每次代码改动后的必需门禁 |
| `spec-check` | `go run ./cmd/spec-check` | Specs 文档与仓库结构一致性 |
| `spec-init` | `sh scripts/spec-init.sh $(VERSION)` | 由模版生成版本技术方案 |
| `lint` / `vuln` | `golangci-lint run --timeout 5m` / `govulncheck ./...` | 可选加强，未安装跳过 |
| `smoke` | `bash scripts/smoke.sh` | 真实二进制冒烟 |

`check` 是其中唯一的复合目标：

```makefile
# Makefile
check: vet test spec-check
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:" && gofmt -l . && exit 1)
```

make 先依次完成 `vet`、`test`、`spec-check` 三个前置目标，任一失败就停；三者都过了才执行配方里的 `gofmt -l .`，有未格式化文件时打印 `gofmt needed:` 与文件列表并以 1 退出。`.ai/ai-rules.md` 的「验证命令」表把它标成「每次代码改动后（必需）」。`make test-integration`、`make test-race`、`make smoke` 没有进 `check`，规则把它们分别绑到版本自验与外部依赖、并发改动、接口改动。`check` 始终是最轻也最常跑的那条。

`lint` 与 `vuln` 用 `command -v` 探测工具，缺失时打印「未安装，跳过（可选）」。这一行以 `||` 收尾：工具装了、也报出了问题，目标本身照样以 0 退出。它们的价值在输出里，不在退出码里，规则也只把它们归为「版本自验（可选）」。

`spec-init` 在 Makefile 层先拦住空 `VERSION`，退出码 2，再交给 `scripts/spec-init.sh`。脚本按顺序检查：版本号必须是 `x.y.z`，否则退出码 2；`Specs/requirements/x.y.z/需求.md` 必须已存在；模版 `Specs/technical/技术方案模版.md` 必须存在；目标文件不得已存在。后三项都是退出码 1。检查完用 `sed` 把模版中的 `{version}` 替换后写入，打印 `created: Specs/technical/x.y.z/技术方案.md`。1.0.0 实录里的两条负例与此一致：需求不存在退出码 1，版本号 `v1` 退出码 2。脚本只创建技术方案，人工需求一个字都不碰，这是「Spec 落地」讲的信息所有权在命令层的样子；整个入口见「spec-init」。

`scripts/` 下第三个脚本 `check-format.sh` 不对应任何 make 目标。调用它的是 `.claude/settings.json` 里 matcher 为 `Edit|Write` 的 `PostToolUse` Hook，对 `cmd/` 与 `internal/` 下的 Go 文件跑 `gofmt -l`，有问题时向 stderr 打印 `gofmt needed:` 并以 2 退出。脚本注释自己写着「只是即时反馈，不替代 make check」。探针验证结果与真实会话中的自动触发尚未记录，「Subagent 与 Hook 落地」如实说明这条边界。

## smoke.sh 证明的是「真实二进制能起来并按契约应答」

`make smoke` 与单元、集成测试的区别在进程身份。脚本先 `go build -o bin/api-smoke ./cmd/api`，再以 `HTTP_ADDR=:18080` 后台启动这个二进制，端口可用 `SMOKE_PORT` 覆盖。`trap` 负责在退出时杀进程、删除二进制。启动后最多轮询 50 次、每次 0.1 秒等 `/healthz` 可达。进程不在了就打印「FAIL: 服务未能启动（端口 18080 可能被占用）」并以 1 退出，其中端口号随 `SMOKE_PORT` 变化。

之后每一项检查走同一个 `check` 函数：发起 curl，把状态码与响应体拆开，状态码必须相等，响应体必须包含给定子串。当前共九项：

| 序号 | 请求 | 期望状态 | 期望响应包含 |
|---|---|---|---|
| 1 | `GET /healthz` | 200 | `"status":"ok"` |
| 2 | `GET /v1/ping` | 200 | `"message":"pong"` |
| 3 | `POST /v1/ping` | 405 | 不检查响应体 |
| 4 | `GET /not-found` | 404 | 不检查响应体 |
| 5 | `POST /v1/notes`，体 `{"title":"smoke","content":"c"}` | 201 | `"id":"n_` |
| 6 | `GET /v1/notes/{id}` | 200 | `"title":"smoke"` |
| 7 | `GET /v1/notes/n_000000000000` | 404 | `"code":"not_found"` |
| 8 | `POST /v1/notes`，体 `{"title":"   "}` | 400 | `"code":"invalid_argument"` |
| 9 | `DELETE /v1/notes/{id}` | 405 | 不检查响应体 |

第 6 与第 9 项用到的 `{id}`，来自脚本在第 5 项之前单独发的一次创建请求，用 `sed` 从响应里取出。任一项失败只标记 `fail=1`，剩下的照跑完，最后打印 `smoke FAILED` 并以 1 退出；全部通过则打印 `smoke passed`。

九项检查的粒度是故意粗的：只看状态码和一个子串，不比对完整响应。它回答装配、路由、中间件和存储在真实进程里有没有接上，不回答每条业务规则对不对。后者归两层测试：`internal/note/service_test.go` 管业务规则，`internal/httpapi/notes_test.go` 管状态码与错误信封。规则里那句「手动 curl 或 Postman 通过但没有对应测试的行为，不算已验证」，对 smoke 一样成立。四层测试各自证明什么，见「测试分层」。

## spec-check 用十条规则守文档，而不是守代码

`cmd/spec-check` 只有一个 `-root` 参数，默认 `.`，调用 `speccheck.Run(root)` 拿到问题列表。检查分四步：必需文件、符号链接、版本目录、Postman 集合。任一步遇到真正的 I/O 故障就整体中止，不把故障算成规则违反。规则共十条：

| 规则 | 检查内容 | 违反时的提示 |
|---|---|---|
| 1 | 需求模版、协议与数据、技术讲解、技术方案模版、`.ai/ai-rules.md`、`Makefile` 存在且是普通文件 | `缺少必需文件` / `不是普通文件` |
| 2 | `ai-rules.md` 的「文件权限规则」「AI 工作流（版本开发）」「验证命令」各恰好一次 | `缺少章节「…」` / `章节「…」重复 N 次` |
| 3 | 技术讲解含「目录结构」「接口清单」「测试策略」「已知问题与待优化项」 | `缺少章节「…」` |
| 4 | `CLAUDE.md`、`AGENTS.md`、`.claude/skills`、`.agents/skills` 是符号链接，目标逐字相等且存在 | `缺少符号链接，应指向 …` / `不是符号链接，应指向 …` / `符号链接目标应为 …` / `符号链接目标不存在: …` |
| 5 | 两侧版本目录名匹配 `x.y.z`；有需求必须有技术方案，反之亦然 | `版本条目必须是目录` / `版本目录名必须是 x.y.z 形式` / `缺少技术方案，运行 make spec-init VERSION=…` / `没有对应人工需求` |
| 6 | 需求至少一个 `F-<v>-NNN`，版本段与目录一致，每个都被技术方案引用 | `至少一个 Feature ID（F-<v>-NNN）` / `技术方案未引用 …` |
| 7 | 技术方案八个二级标题（需求摘要、API 契约、模块设计、测试计划、风险与回滚、文件清单、交付状态、变更记录）各恰好一次 | 同规则 2 |
| 8 | 「交付状态」块能被 `specdoc.ParseDeliveryStatus` 解析并通过门禁 | 解析错误原文 |
| 9 | `api/postman/*.postman_collection.json` 是 JSON 对象，`info.name` 非空字符串，`item` 为数组；目录不存在时跳过 | `顶层必须是 JSON 对象` 等 |
| 10 | 需求侧版本目录只允许 `需求.md`，技术侧只允许 `技术方案.md` 与 `graph.json`，点文件两侧都跳过 | `意外文件` |

标题计数跳过围栏代码块内的行，文档里把 `## 交付状态` 放进代码块当示例不会被误算。Feature ID 的匹配也做了边界处理：`REF-1.0.0-001` 这类紧贴字母的前缀不算，`-0002` 这类超过三位的编号也不算。

输出格式固定。有问题时每行一条 `路径: 问题`，先按路径排、再按问题文本排，末尾一行 `spec-check: N problem(s)`，退出码 1。没有问题时输出 `spec-check ok (N version(s))`，N 是两个 Specs 目录下合法版本目录名的并集大小，退出码 0。参数错误或 I/O 故障退出码 2，故障信息以 `spec-check: …` 写到 stderr。1.0.0 实录里只创建需求、还没有技术方案时，看到的是第一种：

```text
Specs/technical/1.0.0/技术方案.md: 缺少技术方案，运行 make spec-init VERSION=1.0.0
spec-check: 1 problem(s)
```

跑完 `make spec-init VERSION=1.0.0`，在「需求摘要」引用 `F-1.0.0-001` 与 `F-1.0.0-002`，同一条命令输出 `spec-check ok (1 version(s))`。

把它放进 `make check`，是因为「Spec 落地」讲的信息所有权需要一个机器守卫。人工需求由人维护，技术方案由 AI 维护，两者对不对得上不能靠 Agent 自述。规则 6 让每个功能编号必须出现在技术方案里，规则 5 让技术方案不能凭空存在。规则 8 守住唯一一处机器可读的用户决定：`specdoc` 的 `Validate` 规定 `stage: delivered` 必须同时满足 `user_acceptance: confirmed`，且 `review` 为 `pass` 或 `not_required`。Agent 若提前在文档里写下 delivered，下一次 `make check` 就失败，而 `make check` 每次代码改动后都要跑。边界也在这里：它核对结构与引用，不读代码。技术方案说的模块是否真的存在、测试计划是否落地，仍由测试与审查回答。

## spec-graph 是可选命令，不在门禁里

`go run ./cmd/spec-graph <init|status|record|finding|event|check> {version}` 只在启用 Skill `spec-graph-workflow` 时使用，`make check` 不调用它。它把阶段、证据摘要与 finding 记录到 `Specs/technical/{version}/graph.json`，退出码约定为 0 成功、1 check 有问题或未初始化、2 用法错误、3 守卫拒绝或 revision / 锁冲突。1.0.0 实录里最有代表性的一次是 `event 1.0.0 verified` 被拒绝、退出码 3，提示为「守卫不满足: verified: 技术方案「交付状态」user_acceptance 为 pending，需要 confirmed」。Graph 读的是同一个交付状态块，不另存一份完成真相。命令细节与 Subagent 编排见「Spec + Graph 落地」。

## 1.0.0 的门禁输出

在 Go 1.27.0 darwin/arm64 上对当前候选逐条执行，go.mod 声明 `go 1.25.0`，module 是 `example.com/go-backend-template`，仅标准库：

| 命令 | 结果 |
|---|---|
| `make check` | 通过；`spec-check ok (1 version(s))` |
| `make test-integration` | 通过 |
| `make test-race` | 通过 |
| `make smoke` | 九项全部 `OK`，`smoke passed` |
| `make lint` / `make vuln` | golangci-lint / govulncheck 未安装，按设计跳过 |
| Skill `api-verify`（端口 18090） | 17 项场景通过，服务器日志不含笔记正文，端口释放、运行目录删除 |
| Subagent `spec-reviewer` | 第一轮 `changes_required` 5 条 finding，修复后第二轮 `pass`，新增 R6 由 Controller 修正，6 条全部 verified |
| `go run ./cmd/spec-graph check 1.0.0` | 退出码 0 |

这一整列结果没有把 1.0.0 变成已交付。`Specs/technical/1.0.0/技术方案.md` 的交付状态仍是 `stage: verifying`、`user_acceptance: pending`、`review: pass`。`spec-check` 接受这个组合，门禁只在 `delivered` 时触发。命令能证明的到此为止：当前候选通过了全部确定性检查。用户接不接受这个行为是另一回事，只有用户在 6.2 明确确认后，`user_acceptance` 才能改为 `confirmed`，之后才有 `event verified`、Step 7 回写、`stage: delivered` 与再一次 `make check`。绿色不等于交付，这是「验证闭环」的主题。

回到开头的三类。下一个接手 1.0.0 的会话该先重跑 `make check`，再决定信不信文档里的「通过」。make 目标与脚本给出可重跑的退出码，`spec-check` 保证文档没有替用户做决定，斜杠命令和 Subagent 给的是过程与判断。
