---
title: Graph：状态与证据怎样编排
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - concept
  - spec-coding
---

# Graph：状态与证据怎样编排

## 概念

测试回答的是一个问题：某个 Case 在这次运行里通没通过。这个答案没有有效期。需求改一条验收标准、技术方案回写一段、代码多改一个文件之后，昨天那条绿色记录还挂在那里，但它覆盖的已经是旧输入。

Graph 回答的是另一个问题：上游变了之后，哪些结论还算数。

四个约定支撑这个回答。

节点是阶段。一个版本在生命周期里处于哪一段，是一个有限取值，不是一段自然语言描述。

边是事件加守卫。阶段不会自己漂移，只能由一个具名事件推进，事件能不能发生由守卫判定。守卫不满足时事件不落盘，阶段不变，命令失败。

证据绑定候选身份。记录一次门禁通过时，同时记下当时输入文件和候选代码的摘要。证据说的是「在这份输入和这份候选上退出码为 0」，不是「通过了」。

失效是算出来的，不是记下来的。没有谁去把一条结论标记为过期。每次读取重新扫描当前文件、重算摘要、和证据自带的摘要比较，不一致就是不再有效。这样没有人能手工把失效改回有效。

## 本工程怎么落地

入口是 `cmd/spec-graph/main.go`，一个薄 CLI，逻辑全在 `internal/specgraph`：`graph.go` 定义状态结构，`digest.go` 算摘要，`transition.go` 放转换表与守卫，`store.go` 负责锁与落盘，`check.go` 做只读核对。状态文件是 `Specs/technical/<version>/graph.json`。

`graph.go` 的 `State` 定义了全部字段：

| 字段 | 内容 |
| --- | --- |
| `version` | 与目录名一致，不一致时读取直接拒绝 |
| `revision` | 每次成功写入加一，`init` 为 0 |
| `stage` | 六个阶段之一 |
| `inputs` / `candidate` | `files`（路径到 sha256）与 `digest` |
| `findings[]` | `id`、`severity`（P0 到 P3）、`status`、`note`、`updated_at` |
| `evidence[]` | `kind`、`candidate`、`inputs`、`exit_code`、`log_sha256`、`recorded_at`，只追加 |
| `events[]` | `id`、`type`、`from`、`to`、`revision`、`at` |

`kind` 限六种：`check`、`test-integration`、`test-race`、`smoke`、`api-verify`、`review`。finding 的流向是 `open` 到 `accepted` 或 `rejected`，`accepted` 到 `fixed`，`fixed` 到 `verified`。时间统一写成 UTC 秒精度带 `Z` 的 RFC3339。

摘要范围定在 `digest.go`。`inputs` 三个文件：`Specs/requirements/<version>/需求.md`、`Specs/requirements/协议与数据.md`、`Specs/technical/<version>/技术方案.md`。`candidate` 收 `cmd`、`internal`、`scripts`、`api`、`migrations` 五个目录下的普通文件，加 `go.mod`、`go.sum`、`Makefile`；目录不存在跳过，`go.mod` 和 `go.sum` 缺失不算，`Makefile` 必须是普通文件；以 `.` 开头的目录与文件跳过，符号链接不跟随。`digest` 是路径排序后按「路径\n摘要\n」拼接再取一次 sha256。

技术方案有一处例外：先剔除「## 交付状态」标题到其后第一个代码块结束的内容，再算摘要，逻辑在 `stripDeliveryBlock`。这块记录用户决定，守卫要直接读它；不剔，把 `user_acceptance` 从 `pending` 改成 `confirmed` 这个动作本身就会让 inputs 漂移，`verified` 永远过不去。用 `shasum` 手工核对这个文件时也要先剔掉。

六个阶段：`planning`、`implementing`、`reviewing`、`fixing`、`verifying`、`ready_to_deliver`。七类事件定义在 `transition.go` 的 `transitions` 表：

```text
planning --plan_confirmed--> implementing --implementation_done--> reviewing
reviewing --review_passed--> verifying --verified--> ready_to_deliver
reviewing --review_failed--> fixing --fix_done--> reviewing
ready_to_deliver / verifying --readiness_invalidated--> fixing
```

每个守卫只读三类输入：技术方案「交付状态」块、graph.json 里带摘要的证据、finding 状态。

- `plan_confirmed`：交付状态 `stage` 不再是 `planning`。
- `implementation_done`：`check` 证据 `exit_code=0` 且绑定当前 candidate 与 inputs。
- `review_failed`：至少一条 finding 为 `open` 或 `accepted`。
- `fix_done`：无 `open` finding，`check` 证据 `exit_code=0` 且重新绑定当前 candidate 与 inputs。
- `review_passed`：无 `open`、`accepted`、`fixed` finding，`review` 证据 `exit_code=0` 且绑定当前 candidate，不要求 inputs。
- `verified`：`check`、`test-integration`、`smoke`、`api-verify` 四类都 `exit_code=0` 且绑定当前 candidate 与 inputs；若记过 `test-race` 则其 `exit_code` 为 0；所有 finding 为 `verified` 或 `rejected`；交付状态 `user_acceptance` 为 `confirmed`。
- `readiness_invalidated`：`invalidated` 为真。

`invalidated` 由 `transition.go` 的同名函数算出：`stage` 落在 `reviewing`、`verifying`、`ready_to_deliver` 三者之一，且 inputs 或 candidate 摘要与上次写入时绑定的不同。`implementing` 和 `fixing` 本来就在改代码，摘要漂移是预期行为，不算失效。

`record`、`finding`、`event` 三个写入命令都在 `graph.json.lock` 独占锁内执行。锁用 `O_CREATE|O_EXCL` 创建，覆盖 load 到 commit 全程，被占用时退出 3 并打印锁的创建时间。`--expect-revision` 是读后比较，防不住丢写，防丢写的是锁。落盘先 `validateState`，在同目录写 `graph.json.tmp-*`（0644）并 `fsync`，`init` 用 `os.Link` 独占创建、文件系统不支持硬链接时退回 rename，其余情况 rename 覆盖，失败删临时文件。读取拒绝首个 JSON 值之后的多余内容与重复键，`DisallowUnknownFields`，最后把事件序列从 `planning` 重放一遍，重放结果与记录的 `stage` 不一致就拒绝。所以 graph.json 不手工编辑。

退出码四档：0 成功或 `check` 无问题；1 是 `check` 报告问题、graph.json 不存在、其他读写与解析错误；2 是用法错误，包括子命令、版本号、事件类型、`--kind`、`--severity`、`--status` 不合法与缺少必填 flag；3 是守卫拒绝与冲突，包括非法转换、守卫不满足、finding 流不允许、锁被占用、`--expect-revision` 不符。

`status` 和 `check` 分工不同。`status` 只读当前状态，重算摘要后输出 `stage`、`revision`、`invalidated`、`drifted_inputs`、`drifted_candidate`、finding 计数、各 kind 最新证据和交付状态，给人看现在处在哪里，文本模式摘要只显示前 12 位。`check` 做跨文件一致性核对，按固定顺序报告解析失败、残留锁文件、有 graph.json 却缺技术方案、摘要无法计算、`ready_to_deliver` 且已失效、交付状态无法解析、交付状态为 `delivered` 但 graph 不在 `ready_to_deliver` 或已失效、graph 已 `ready_to_deliver` 但 `user_acceptance` 不是 `confirmed`，有问题按「路径: 消息」打印并退出 1。

用户决定不进 Graph。`State` 里没有任何字段表示用户确认，守卫需要时调 `internal/specdoc` 的 `ParseDeliveryStatus` 现读技术方案的「交付状态」块，取 `stage`、`user_acceptance`、`review` 三个值。`ready_to_deliver` 只表示守卫放行，交付由交付状态的 `delivered` 表示，`check` 核对两者一致。

Skill `.ai/skills/spec-graph-workflow/SKILL.md` 把 Step A 到 E 叠在七步工作流上：A 落在 Step 4，`init` 加 `event plan_confirmed`；B 落在 Step 5，跑 `make check` 并 `record`；C 落在 Step 6.1，登记 finding 与 `review` 证据；D 落在 Step 6.1 与 6.2，逐项 `record` 后等用户确认再 `event verified`；E 落在 Step 7 及之后，`spec-graph check` 与 `readiness_invalidated`。

## 扩展思路

**把 CI 结果记进来。** `record --kind <k> --exit <n> --log <path>` 的三个参数在 CI 里都拿得到：`--exit` 取作业退出码，`--log` 指向作业日志，`--kind` 按门禁类型选。候选摘要不来自参数，而是扫描检出的工作区，必要时用 `--root` 指定目录。CI 跑完在同一 checkout 上执行 `record`，摘要就绑定这次构建的候选。要点是 CLI 无法验证 `--exit` 是不是来自真实运行，这条诚实义务必须由调用方守住——脚本里让 `record` 直接消费上一条命令的 `$?`，不要写死。

**多候选并行。** 状态文件按版本分目录，不同版本天然不互相干扰。同一版本上多条分支并行时，`--root` 指向各自的工作区副本即可隔离：摘要是对 `--root` 下的文件算的，锁也在各自的 `graph.json` 旁边。要合并时，先在合并后的树上重跑门禁并 `record`，因为合并本身改变了候选摘要，任何一侧的旧证据都不再绑定当前 candidate。

**接到发布流程。** 发布脚本的前置条件可以是 `spec-graph check` 退出 0 加上 `status --json` 里 `stage` 为 `ready_to_deliver`、`invalidated` 为 `false`。`--json` 输出是稳定结构，管道里取值不用解析文本。再往前一步，可以把发布产物的摘要也记成一条证据 kind，让「发过的这个包」和「验过的这份候选」对得上。

**什么时候不启用。** Graph 有固定成本：每次门禁多一条 `record`，每次阶段推进多一条 `event`，改完文档要重跑门禁重记证据。日常小修改按 `.ai/ai-rules.md` 的「日常代码修改」一节走就够了，`make check` 加代码评审能覆盖。单人单分支、需求在开发期间不动、没有独立审查环节的版本，Graph 记的东西和你脑子里的东西一样多。真正值回成本的场景在 Skill 描述里写死了：版本较大、需要独立审查、需要在需求或代码变化后判断哪些证据已失效。第三条是关键——如果你从没遇到过「这轮测试还算不算数」这个问题，就还不需要它。
