---
title: Spec + Graph 落地：spec-graph CLI、graph.json 与 Subagent 编排
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - spec-graph
  - cli
  - subagent
  - controller
---

# Spec + Graph 落地：spec-graph CLI、graph.json 与 Subagent 编排

Graph 在 go-backend-template 里只有一个落点：`Specs/technical/<version>/graph.json`。维护它的是 `cmd/spec-graph` 这个薄 CLI，逻辑全在 `internal/specgraph` 的五个文件里。它不运行任何命令，做的只有三件确定性的事：记录阶段，把每条证据绑定到当时的 inputs 与 candidate 摘要，用守卫决定一个事件能不能发生。「Spec + Graph 理论」里那句「Loop 管局部收敛，Graph 管结论是否仍然有效」，落到代码就是 `boundEvidence` 拿证据里的摘要跟当前重新计算的摘要比对。

`1.0.0` 演示版本真实走完了这条链：46 次 revision，两个候选，六条 finding，最后停在 `verifying`。`verified` 事件的守卫读到技术方案「交付状态」里 `user_acceptance` 还是 `pending`。版本没有交付，Graph 也不替用户做这个决定。

## graph.json 只有八个顶层字段

文件形状由 `internal/specgraph/graph.go` 里的 `State` 决定。写盘时 `findings`、`evidence`、`events` 为空也输出 `[]`，不出现 `null`：

| 字段 | 类型 | 含义 |
|---|---|---|
| `version` | string | 与目录名一致，读取时不一致直接拒绝 |
| `revision` | int | 每次成功写入加 1，`init` 为 0 |
| `stage` | string | `planning`、`implementing`、`reviewing`、`fixing`、`verifying`、`ready_to_deliver` 之一 |
| `inputs` | Snapshot | 需求与方案的 `files`（路径到 sha256）与 `digest` |
| `candidate` | Snapshot | 代码与脚本的 `files` 与 `digest` |
| `findings` | []Finding | `id`、`severity`（P0..P3）、`status`、`note`、`updated_at` |
| `evidence` | []Evidence | `kind`、`candidate`、`inputs`、`exit_code`、`log_sha256`、`recorded_at`，只追加 |
| `events` | []Event | `id`、`type`、`from`、`to`、`revision`、`at` |

两个 Snapshot 的文件集合固定写在 `internal/specgraph/digest.go`。inputs 是三个文件：`Specs/requirements/<version>/需求.md`、`Specs/requirements/协议与数据.md`、`Specs/technical/<version>/技术方案.md`。candidate 扫描 `cmd`、`internal`、`scripts`、`api`、`migrations` 五个目录，加上 `go.mod`、`go.sum` 和 `Makefile`；目录不存在就跳过，`go.mod`、`go.sum` 缺失不算，`Makefile` 必须是普通文件。扫描只收普通文件，以 `.` 开头的目录或文件一律跳过，符号链接不跟随。`digest` 把路径排序，按「路径、换行、摘要、换行」拼接，再取 sha256。`1.0.0` 的 candidate 有 37 个文件，没有 `go.sum` 和 `migrations` 条目。

技术方案是唯一的例外。先用 `stripDeliveryBlock` 剔除「## 交付状态」标题到其后第一个代码块结束的内容，再取 sha256。`digest.go` 的注释写了理由：交付状态块记录的是用户决定，守卫直接读它。按整文件摘要的话，Step D 记完证据再把 `user_acceptance` 改成 `confirmed`、Step E 改成 `delivered`，两次改动都会让 inputs 漂移，已记录的证据不再满足 `verified` 守卫与 `check` 的 `delivered` 规则。用 `shasum` 手工核对这个文件，同样要先剔除此块。

## 六个子命令、四个退出码

`cmd/spec-graph/main.go` 的 `usageText` 就是完整的参数表：

```text
用法: spec-graph [--root <dir>] <子命令> <version> [参数]
  init <version>
  status <version> [--json]
  record <version> --kind <k> --exit <n> [--log <path>] [--expect-revision <n>]
  finding <version> --id <id> --severity <P0..P3> --status <s> [--note <text>] [--expect-revision <n>]
  event <version> <type> [--id <id>] [--expect-revision <n>] [--at <RFC3339>]
  check <version>
```

`--root` 只在子命令之前解析，版本号必须形如 `x.y.z`。`record` 的 `--kind` 限于 `check`、`test-integration`、`test-race`、`smoke`、`api-verify`、`review` 六种，给了 `--log` 就读取文件算 sha256 写入 `log_sha256`。`event` 的 `--at` 只改写事件时间戳，必须是 RFC3339。`init`、`record`、`finding`、`event` 成功时打印 `stage=<stage> revision=<n>`。

| 退出码 | 触发条件 |
|---|---|
| 0 | 成功；`check` 没有问题 |
| 1 | `check` 报告问题；`graph.json` 不存在（未 `init`）；其他读写或解析错误 |
| 2 | 用法错误：子命令、版本号、事件类型、`--kind`、`--severity`、`--status` 不合法，缺少必填 flag，多余参数，`--log` 读不到 |
| 3 | 守卫拒绝：非法转换或守卫不满足、finding 状态流不允许、新 finding 不是 `open`；冲突：锁被占用、`--expect-revision` 不符、事件 `--id` 类型冲突、自动 id 被占、`init` 时文件已存在 |

写入路径有三层保护。锁与落盘在 `internal/specgraph/store.go`，`--expect-revision` 的比较在 `internal/specgraph/transition.go` 的 `loadExpecting`。

第一层是 `graph.json.lock`。`record`、`finding`、`event` 先用 `O_CREATE|O_EXCL` 独占创建锁文件，覆盖 load 到 commit 全程；锁已存在就以退出码 3 拒绝，并打印锁的创建时间。进程被 `SIGINT` 或 `SIGKILL` 打断时 `defer` 不会执行，残留的锁由 `check` 报告，确认无进程运行后手工删除。第二层是 `--expect-revision`，读后比较当前 revision，不符退出 3；注释里写明它防不住丢写，防丢写的是锁。第三层是落盘：先 `validateState`，再在同目录创建 `graph.json.tmp-*`，`Chmod(0644)`、写入、`fsync`；`init` 用 `os.Link` 独占创建目标，文件系统不支持硬链接时退回 rename，其余用 `rename` 覆盖。任一步失败都删掉临时文件。

读取一侧同样严格。`checkJSONShape` 拒绝两类 `encoding/json` 会静默接受的输入：首个 JSON 值之后的多余内容，同一对象内的重复键。之后用 `DisallowUnknownFields` 解码，再 `validateState`：`version` 必须与目录一致，stage、severity、status、kind 都必须在枚举内，时间必须是 UTC 秒精度带 `Z` 的 RFC3339，finding 与 event 的 id 不能为空或重复，事件序列要能从 `planning` 逐条重放到记录的 `stage`。手工改 `graph.json` 改坏任何一处，下一次命令就以「解析 graph.json」或「graph.json 校验失败」拒绝，`check` 把它作为问题报告并退出 1。

幂等规则有两条。`event --id` 遇到同 id 同类型返回成功且不写盘、revision 不变，同 id 不同类型退出 3；不给 `--id` 时自动生成 `evt-<revision>`，被占用也退出 3。`finding` 同 id 相同 `--status` 是幂等成功，不改 severity 与 note；合法转换不带 `--note` 时保留原 note，避免 `open` 时登记的审查原因被 `accepted` 清空。每次成功写入都把当下的 inputs 与 candidate 摘要回写到顶层，所以 `status` 里的漂移是相对上一次写入而言的。

## 每个事件的守卫读三类输入

`internal/specgraph/transition.go` 的 `transitions` 表就是全部状态机。按 Skill 的说法，守卫读三类输入：记录用户决定的技术方案「交付状态」、`graph.json` 里绑定摘要的证据、finding 状态。代码里的 `guardContext` 只持有 `Graph`、`State` 与当前重算的 inputs、candidate 两个 `Snapshot`。证据与 finding 都在 `State` 里；交付状态不是字段，守卫通过 `c.delivery()` 现读技术方案。`boundEvidence(kind, needInputs)` 要求该 kind 最后一条证据 `exit_code=0`、`candidate` 等于当前重算的摘要，`needInputs` 为真时 `inputs` 也要相等。

| 事件 | from -> to | 守卫 |
|---|---|---|
| `plan_confirmed` | `planning` -> `implementing` | 交付状态 `stage` 不再是 `planning` |
| `implementation_done` | `implementing` -> `reviewing` | `check` 证据绑定当前 candidate 与 inputs |
| `review_failed` | `reviewing` -> `fixing` | 存在 `open` 或 `accepted` 的 finding |
| `review_passed` | `reviewing` -> `verifying` | 没有 `open`、`accepted`、`fixed` 的 finding；`review` 证据绑定当前 candidate（不要求 inputs） |
| `fix_done` | `fixing` -> `reviewing` | 没有 `open` 的 finding；`check` 证据绑定当前 candidate 与 inputs |
| `verified` | `verifying` -> `ready_to_deliver` | `check`、`test-integration`、`smoke`、`api-verify` 四类都绑定当前 candidate 与 inputs；`test-race` 若有记录须 `exit_code=0`；所有 finding 为 `verified` 或 `rejected`；交付状态 `user_acceptance` 为 `confirmed` |
| `readiness_invalidated` | `verifying` 或 `ready_to_deliver` -> `fixing` | `invalidated` 为真 |

`invalidated` 的定义只有一行：stage 在 `reviewing`、`verifying`、`ready_to_deliver` 之一，且 inputs 或 candidate 摘要与上一次写入不同。finding 的状态流也是固定的：`open` 到 `accepted` 或 `rejected`，`accepted` 到 `fixed`，`fixed` 到 `verified`。

## check 核对一致性，status 只报告

`Check` 在 `internal/specgraph/check.go`，只读，按顺序报告：`graph.json` 解析或校验失败；锁文件残留；有 `graph.json` 但缺技术方案；摘要无法计算；stage 为 `ready_to_deliver` 且已失效，提示执行 `readiness_invalidated`；交付状态无法解析；交付状态为 `delivered` 但 graph 不在 `ready_to_deliver` 或已失效；graph 已 `ready_to_deliver` 但 `user_acceptance` 不是 `confirmed`。每条按 `路径: 消息` 打印，有问题退出 1。最后两条把 Skill 里那句「不把 `ready_to_deliver` 当作交付」变成了机器规则：交付由技术方案「交付状态」表示，两边必须一致。

`Status` 每次调用都重算摘要，输出 `version`、`stage`、`revision`、`invalidated`、`drifted_inputs`、`drifted_candidate`、`finding_counts`（按状态计数）、`latest_evidence`（六种 kind 各取最后一条，文本模式只显示 candidate 与 inputs 前 12 位）和 `delivery` 三键。`--json` 输出同样的结构，`delivery` 展开为 `stage`、`user_acceptance`、`review`，不做 HTML 转义。

## Skill 的五步：主会话是唯一 Controller

`.ai/skills/spec-graph-workflow/SKILL.md` 是可选工作流，前提是已按 `.ai/ai-rules.md` 完成 Step 1 到 3。它的前提说明把 `docs/spec-graph/理论.md` 列为理论与边界入口，把 `docs/spec-graph/落地.md` 列为命令细节入口。后者是工程内的速查版本，含命令速查表与事件守卫表。SKILL.md 把角色分成四份：

| 角色 | 承担者 | 只做 |
|---|---|---|
| Controller | 主会话 | 读需求与方案、切片、委派、作为唯一执行者运行 `make` 门禁与 `api-verify`、用 `spec-graph` 记录证据与推进阶段、回写文档、与用户交互 |
| Implementer | Subagent `spec-implementer` | 按切片测试先行写代码与测试，只跑 focused 测试，返回改动与 RED/GREEN 证据 |
| Reviewer | Subagent `spec-reviewer` | 只读三轮审查，返回 verdict 与 findings |
| 状态与证据 | `cmd/spec-graph` | 确定性阶段转换、守卫、证据身份与失效判定；不运行任何命令 |

Step A 在用户确认技术方案后把交付状态 `stage` 改为 `implementing`，`init` 建图，`event plan_confirmed`。

Step B 按技术方案「文件清单」与 Feature 切片。互不重叠的切片可以同时委派多个 `spec-implementer`，每个给出版本、Feature 与 AC、允许路径和禁止路径。收回 YAML 后由 Controller 运行 `make check` 并 `record --kind check`，再 `event implementation_done`。

Step C 委派 `spec-reviewer`，逐条 `finding --status open` 登记，Controller 判断 `accepted` 或 `rejected`，需求取舍类先问用户。verdict `pass` 记 `record --kind review --exit 0`，`changes_required` 记 `--exit 1`。有待处理 finding 就 `review_failed` 进入 `fixing`，修复后 `fixed`、重跑 `make check`、`fix_done` 回到 `reviewing` 再审，复审关闭的记 `verified`。没有阻塞 finding 且 review 证据为 0 时 `review_passed`，并把交付状态 `review` 写成 `pass`。

Step D 由 Controller 依次运行 `make test-integration`、`make test-race`（有并发改动时）、`make smoke`、Skill `api-verify` 并逐项 `record`，交付状态 `stage` 改 `verifying`，用户明确确认后改 `user_acceptance: confirmed`，再 `event verified`。

Step E 按 Step 7 回写文档，交付状态改 `delivered`，运行 `make check` 与 `spec-graph check`。之后任何需求、协议、方案或代码改动都会让 `status` 显示 `invalidated: true`，`readiness_invalidated` 回到 `fixing`，旧证据保留为历史但不再满足守卫。

分工里最重要的一条是 record 的诚实义务。`spec-graph` 只能核对退出码是整数、日志文件存在，无法知道那个退出码是不是刚跑出来的。Skill 的「不要做的事」把这一条写成硬规则：`record --exit 0` 不是凭空宣称，退出码必须来自 Controller 刚运行的命令，`--log` 指向其输出；Implementer 不得运行 broad 门禁或修改 `Specs/requirements/**`，Reviewer 不修改任何文件。`.claude/agents/spec-implementer.md` 的输入约束再把 `Specs/requirements/**`、`graph.json` 和 `.ai/skills/go-development/**` 列为始终禁止修改的路径。Subagent 的结论不替代命令结果，命令由唯一 Controller 执行，证据由 Graph 绑定身份，三者各管一段。Codex 没有 Subagent，可以只用 CLI 记录状态，实现与审查在同一会话串行完成。

## 1.0.0 的 graph 记录

`Specs/technical/1.0.0/graph.json` 停在 revision 46、stage `verifying`、`invalidated: false`，五个事件都用自动 id，从 `evt-1` 到 `evt-41`：

| revision | 动作 | 结果 |
|---|---|---|
| 0 | `init` | `planning` |
| 1 | `event plan_confirmed` | `implementing` |
| 2 | `record check` | 候选 `04a68378d8b4` |
| 3 | `event implementation_done` | `reviewing` |
| 4 到 7 | `record test-integration`、`test-race`、`smoke`、`api-verify` | 同一候选 |
| 8 | `record review --exit 1` | 第一轮 `changes_required` |
| 9 到 13 | R1 到 R5 `open` | R1、R2、R5 为 P2，R3、R4 为 P3 |
| 14 | `event review_failed` | `fixing` |
| 15 到 24 | R1 到 R5 `accepted`，再 `fixed` | R1、R2 由 `spec-implementer` 角色测试先行修复，R3 到 R5 由 Controller 回写文档 |
| 25、26 | `record check`，`event fix_done` | 候选 `2d03ec4f1c56`，回到 `reviewing` |
| 27 到 30 | `record` 四类门禁 | inputs `e5636895cabd` |
| 31 到 39 | R6 `open`、`accepted`、`fixed`，R1 到 R6 `verified` | 第二轮 verdict `pass` |
| 40、41 | `record review --exit 0`，`event review_passed` | `verifying`，inputs `1c1fd583b8f5` |
| 42 到 46 | `record` 五类门禁 | 候选 `2d03ec4f1c56`、inputs `1c1fd583b8f5` |

42 到 46 这五条不是重复劳动。27 到 30 的证据绑定 inputs `e5636895cabd`，随后 Controller 修正 R6 改动了技术方案，revision 40 的 review 证据绑定的 inputs 已经是 `1c1fd583b8f5`。R6 是测试计划里一个用例名挂错。`review_passed` 只看 review 证据的 candidate，事件通过；`verified` 要求四类门禁都绑定当前 inputs，Controller 只能在同一候选上重新运行并记录。Graph 判定旧结论是否仍然有效，靠的是摘要不相等，不靠人记得改过什么。

R1 的处理划出了 Controller 的判断边界。审查指出 `memstore.Store.Save` 在 ID 冲突时静默覆盖，需求写的是「不得丢失或覆盖」。返回 `note.ErrAlreadyExists` 是严格满足需求，不是放宽需求，Controller 直接 `accepted`，没有交给用户。R1、R2 的修复新增了 `TestStore_Save_DuplicateID`、`TestService_Create_SaveAlreadyExists`，以及 `TestCreateNote_BadBody` 的 `trailing json value`、`trailing garbage` 两个子用例，每条先 RED 再 GREEN。

最终门禁记录完成后，`go run ./cmd/spec-graph check 1.0.0` 退出码 0。接着的 `event 1.0.0 verified` 被拒绝：

```text
守卫不满足: verified: 技术方案「交付状态」user_acceptance 为 pending，需要 confirmed
```

退出码 3。技术方案交付状态是 `stage: verifying`、`user_acceptance: pending`、`review: pass`。版本没有交付，等用户验收确认。确认后的动作顺序固定：交付状态 `user_acceptance` 改 `confirmed`，`event verified` 进入 `ready_to_deliver`，Step 7 回写，交付状态改 `delivered`，最后 `make check` 与 `spec-graph check` 核对两边一致。

有一处边界必须如实说明。演示里的 `spec-implementer` 角色由通用 Agent 加载 `.claude/agents/spec-implementer.md` 正文扮演，不是 Claude Code 原生 agents 目录加载，`spec-reviewer` 也一样。两个定义文件的 `tools` 字段在真实 Claude Code 会话中的运行时限制没有端到端记录，见「Subagent 与 Hook 落地」。Graph 不依赖这一点：它只认 `graph.json` 里的摘要、退出码和交付状态，谁扮演了角色不改变守卫的判定。
