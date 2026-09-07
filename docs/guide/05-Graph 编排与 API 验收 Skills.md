---
title: Graph 编排与 API 验收 Skills
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - concept
  - spec-coding
---

# Graph 编排与 API 验收 Skills

## 概念

版本开发到了中后期，有两件事每次都长得一样：谁来写、谁来审、谁来跑门禁、证据算不算数；以及接口上线前，逐个路径拿真实进程核对状态码和错误信封。

这两件事都是流程，不是判断。流程写在 Skill 文件里，每次执行按同一份步骤走；留在人的记忆里，就变成每次临场决定跑几条命令、审到哪一层、哪些接口试过了。差别不在某一次做得好不好，而在下一次能不能复现上一次的结论。

编排 Skill 要解决的是角色越界和证据失效：Implementer 顺手跑了全量门禁，Reviewer 顺手改了代码，代码改完之后昨天那份通过记录还算不算数。验收 Skill 要解决的是"测试过了"和"服务真的这么响应"之间的缝隙。

## 本工程怎么落地

### spec-graph-workflow 的角色划分

`.ai/skills/spec-graph-workflow/SKILL.md` 定义四个角色，边界写在表格里：

| 角色 | 承担者 | 边界 |
| --- | --- | --- |
| Controller | 主会话 | 唯一执行者，运行 `make` 门禁与 `api-verify`，用 `spec-graph` 记录证据、推进阶段、回写文档 |
| Implementer | Subagent `spec-implementer` | 按切片测试先行，只跑 focused 测试 |
| Reviewer | Subagent `spec-reviewer` | 只读三轮审查，返回 verdict 与 findings |
| 状态与证据 | `cmd/spec-graph` | 阶段转换、守卫、证据身份与失效判定，不运行任何命令 |

边界由工具授权兜底。`.claude/agents/spec-reviewer.md` 的 frontmatter 里 `tools: Read, Grep, Glob` —— 没有 Edit、Write、Bash，想改文件也没有手段。`.claude/agents/spec-implementer.md` 有 Edit/Write/Bash，但正文第 3 条写死"不运行 `make check`、`make smoke`、`api-verify`"，并列出始终禁止修改的路径：`Specs/requirements/**`、`Specs/technical/{version}/graph.json`、`.ai/skills/go-development/**`。

两个 Subagent 的返回都是固定 YAML。Implementer 返回 `changed_files` / `tests`（每条带 `red_observed` 与 `green_command`）/ `open_questions`；Reviewer 返回 `verdict: pass | changes_required | blocked`、带 `id` / `severity` / `location` / `failure_mode` / `minimum_fix` / `required_verification` 的 findings，以及 `verification_gaps`。verdict 为 `pass` 时也要列 `verification_gaps`，用来标出"仓库里没有证据表明跑过"的检查。

用户决定（方案确认、验收确认、需求取舍类 finding）只记录在技术方案「交付状态」块，Graph 读它作为守卫输入，不另存一份。

### 阶段与事件

```text
planning --plan_confirmed--> implementing --implementation_done--> reviewing
reviewing --review_passed--> verifying --verified--> ready_to_deliver
reviewing --review_failed--> fixing --fix_done--> reviewing
ready_to_deliver / verifying --readiness_invalidated--> fixing
```

`internal/specgraph/store.go` 里的 `evidenceKinds` 固定六类：`check`、`test-integration`、`test-race`、`smoke`、`api-verify`、`review`。每条证据带退出码、日志摘要，以及记录时刻的 candidate 与 inputs 摘要。

### Step A 到 E

- **Step A 初始化**：技术方案确认后把「交付状态」`stage` 改为 `implementing`，`go run ./cmd/spec-graph init {version}` 在 `Specs/technical/{version}/graph.json` 建图，再 `event {version} plan_confirmed`。
- **Step B 实现**：按「文件清单」切片，互不重叠的切片可并行委派多个 Implementer，每个给出允许路径与禁止路径；收回 YAML 后由 Controller 跑 `make check`，`record {version} --kind check --exit <退出码> --log <输出文件>`，再 `event implementation_done`。
- **Step C 审查循环**：findings 逐条 `finding {version} --id R1 --severity P1 --status open`，由 Controller 判 `accepted` / `rejected`（需求取舍类先问用户）。有阻塞项走 `review_failed` 进 `fixing`，修好标 `fixed`、重跑并 `record`、`fix_done` 回 `reviewing`，复审关闭的标 `verified`。
- **Step D 验证**：Controller 依次跑 `make test-integration`、`make test-race`、`make smoke` 和 `api-verify`，各自 `record`；用户确认后把「交付状态」`user_acceptance` 改为 `confirmed`，再 `event verified`。
- **Step E 交付与失效**：`stage` 改 `delivered`，跑 `make check` 与 `spec-graph check {version}`。之后任何需求或代码改动会让 `status` 显示 `invalidated: true`，随后 `event readiness_invalidated` 回到 `fixing`。

守卫不满足时命令以退出码 3 拒绝（`ErrGuard` / `ErrConflict`，参数错误是 2）。`verified` 的守卫在 `internal/specgraph/transition.go` 里要求 `check`、`test-integration`、`smoke`、`api-verify` 四类证据在当前 candidate 上退出码为 0，所有 finding 为 `verified` 或 `rejected`，`user_acceptance` 为 `confirmed`。

证据身份是文件内容的 sha256，技术方案按剔除「## 交付状态」块后的内容取摘要——改交付状态不会误伤已有证据，改代码会。写入在 `graph.json.lock` 独占锁内执行，并发写用 `--expect-revision` 处理；`graph.json` 不手工编辑。

Graph 阶段到 `ready_to_deliver` 不等于交付。交付由「交付状态」`delivered` 表示，`spec-graph check` 会核对两者一致，`make check` 里的 `spec-check` 则会拒绝 `user_acceptance` 未 `confirmed`、或 `review` 还是 `pending` / `changes_required` 的 delivered。

### api-verify 的七个 Step

`.ai/skills/api-verify/SKILL.md` 的定位写在正文第一段：对真实进程做契约核对，不替代单元与集成测试。所以前置校验第 2 条要求 `make check` 与 `make test-integration` 已通过，否则先回去补测试。前置还要确认 `go version` 与 `curl --version` 可用、读取验证来源（传 version 读技术方案「API 契约」，不传读 `api/postman/*.postman_collection.json` 加「接口清单」）、读 `Specs/requirements/协议与数据.md` 拿错误信封格式。

隔离靠两处：端口 `API_VERIFY_PORT:-18090`，和 `scripts/smoke.sh` 的 `SMOKE_PORT:-18080` 错开；运行目录 `RUN_DIR="$(mktemp -d)"`，二进制、`server.log`、`pid` 都放进去，并行跑两次不会互相误杀。

Step 1 启动服务后轮询 `/healthz` 至多 50 次。Step 2 每个接口至少四类场景：成功路径、每种错误场景（核对 `error.code` 且 message 不泄露内部信息）、方法不匹配确认 405、契约声明幂等时的重复提交。Step 3 核对 Postman 集合，缺的记「集合待补」、不一致的记「集合待更新」。

Step 3.5 是这个 Skill 真正的硬门：每个场景都要在 `*_test.go` / `*_integration_test.go` 里找到对应用例并记下用例名，找不到就记「缺测试」，必须补齐后才能给"通过"结论。报告表格里「对应测试」是独立一列：

```
| 接口 | 场景 | 期望 | 实际 | 对应测试 | 结论 |
| POST /v1/users | 重复邮箱 | 409 conflict | 500 internal_error | 缺测试 | 失败 |
```

写着「缺测试」的行结论只能是失败——一个接口跑对了但没有测试守着，下一次改动没人拦。

Step 4 无论结果如何都要 `kill` 加 `rm -rf "$RUN_DIR"`，并用 `lsof -i :$PORT` 确认端口释放。Step 5 出报告，Step 6 处理失败：失败或缺测试都回到代码，先补失败测试再修，`make check` 通过后只重跑失败接口与相关场景；失败原因如果是契约本身不合理，不得私自改契约，按需求变更规则更新技术方案再验证。

这一整套是 Agent 自验，对应 ai-rules 的 Step 6.1。自验通过后仍要进 Step 6.2：用户自己 `make run`，用 Postman 或报告里的 curl 命令验一遍并确认。Postman 是可选工具，用户确认是必需步骤。

## 扩展思路

**换掉验证工具。** api-verify 的七个 Step 是骨架，curl 是其中一种实现。把 Step 2 换成契约测试工具（schema 直接比对）或压测工具（同一批接口跑并发与 p99），Step 3.5 的"每个场景都要指向一个用例名"和 Step 4 的清理义务照旧。骨架里最贵的是这两条，不是 curl。

**接入外部依赖后补前置校验。** 前置第 6 条现在写的是"需要外部依赖时确认依赖已就绪（连接串环境变量、`/readyz` 返回 200）"，但模板的 `internal/httpapi/handler.go` 只有 `GET /healthz`，没有 `/readyz`。接数据库时要一并做三件事：加一个真正探连接的 `/readyz`、把迁移是否到最新版纳入前置、给验证脚本准备可重复的初始数据（否则第二次跑成功路径就会撞唯一约束）。清理义务同步扩大到数据，不只是进程和端口。

**没有 Subagent 的环境。** SKILL 开头已经写了 Codex 的降级路径：只用 `spec-graph` CLI 记录状态，实现与审查在同一会话串行完成。降级掉的是"审查者看不到实现过程"这层独立性，保留的是阶段、守卫和证据失效。想补回一部分独立性，可以让审查在新会话里按 `spec-reviewer.md` 正文执行——上下文换了，至少不会顺着自己刚写的代码读。

**什么时候不该用 Graph。** SKILL 的 description 划了适用范围：版本较大、需要独立审查、需要在需求或代码变化后判断哪些证据已失效。一个 Feature、两三个文件、改完一次门禁就过的版本，`init` / `record` / `event` 的记账成本高于它挡住的风险，直接按 ai-rules 的 Step 1 到 7 走完即可。判断依据是证据会不会失效——改动跨多轮、审查要来回、代码改了还得回头看哪些结论作废，才值得让 Graph 替你记账。
