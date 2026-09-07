---
title: Spec + Graph 理论：Loop 管局部收敛，Graph 管结论是否仍然有效
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - graph-engineering
  - loop
  - evidence
  - invalidation
---

# Spec + Graph 理论：Loop 管局部收敛，Graph 管结论是否仍然有效

`1.0.0` 演示版本第二轮审查结束时，`make check`、`make test-integration`、`make test-race`、`make smoke` 全部通过，`api-verify` 的 17 项场景全部通过，`spec-reviewer` 返回 pass。之后 Controller 按 R6 改掉了技术方案测试计划里挂错的一个用例名。这一改，前面哪些结论还算数？

测试答不了。一条绿色记录只说明某个命令在某份代码、某份文档上成功过；文档或代码再变一次，它就是历史。Loop 把一个 Case 从 RED 收敛到 GREEN（见「测试分层」「验证闭环」），Graph 管的是另一件事：这条结论绑定的输入，还是不是当前输入。CLI 用法与 Subagent 编排见「Spec + Graph 落地」，本篇只讲机制。

## 为什么有了 Spec 和测试还需要 Graph

Spec 划清了信息所有权：`Specs/requirements/**` 只读，`Specs/technical/**` 由 AI 维护，用户决定只写在技术方案「交付状态」块（见「Spec 落地」「验证闭环」）。测试分层证明局部行为成立，`spec-check` 核对文档结构。三者加起来还缺一条关系：某个结论是在哪份需求、哪份方案、哪份代码上得出的。

缺这条关系，上游一变就只能靠人记。需求改了一条验收标准，旧的审查 pass 覆盖的是旧需求。技术方案回写一段，之前记录的门禁通过绑定的是旧方案。代码多改一个文件，`smoke` 的通过就不再属于当前候选。`internal/specgraph` 只做一件事：把每条结论和它当时的输入、候选摘要绑在一起，每次读取重算摘要再比较。

`1.0.0` 的 `graph.json` 里能看到这条关系起作用。修复收尾到第二轮审查结论之前有五条门禁证据，记在 revision 25 与 27 到 30。revision 25 属于 `fixing` 阶段，`fix_done` 是 revision 26，后四条属于 `reviewing` 阶段，五条都绑定 inputs 摘要 `e5636895cabd`。审查后 Controller 回写技术方案，改了 R6 用例名，补了变更记录，inputs 摘要变成 `1c1fd583b8f5`，这五条证据不再与当前输入一致。`review_passed` 之后 Controller 重跑门禁并记为 revision 42 到 46，`verified` 的守卫才有可用证据。摘要取 `status` 输出的前 12 位。

## 图里的六种东西各表示什么

`Specs/technical/<version>/graph.json` 是一个 JSON 文件，全部内容由 `internal/specgraph/graph.go` 的 `State` 定义。

| 概念 | 在 graph.json 里 | 表示什么 |
|---|---|---|
| 节点（阶段） | `stage` | 版本当前处在生命周期的哪一段，六个取值 |
| 边（事件与守卫） | `events[]`：`id`、`type`、`from`、`to`、`revision`、`at` | 一次被守卫放行的阶段转换；守卫不满足时事件不落盘 |
| 证据 | `evidence[]`：`kind`、`candidate`、`inputs`、`exit_code`、`log_sha256`、`recorded_at` | 一次门禁或审查的结果，以及它绑定的候选与输入摘要；`kind` 有 `check`、`test-integration`、`test-race`、`smoke`、`api-verify`、`review` 六种 |
| finding | `findings[]`：`id`、`severity`、`status`、`note`、`updated_at` | 审查发现，严重度 `P0` 到 `P3`，状态从 open 到 accepted 或 rejected，再到 fixed、verified |
| revision | `revision` | 每次成功写入加一，`init` 为 0；`--expect-revision` 用它做比较后写入 |
| 幂等键 | 事件 `--id`、finding `--id` 加相同 `--status` | 重复执行同一次登记得到同一结果，不重复写入；事件未显式给 `--id` 时自动生成 `evt-<revision>` |

六项里证据最要紧，`graph.go` 的定义是：

```go
// internal/specgraph/graph.go
type Evidence struct {
	Kind       string `json:"kind"`
	Candidate  string `json:"candidate"`
	Inputs     string `json:"inputs"`
	ExitCode   int    `json:"exit_code"`
	LogSHA256  string `json:"log_sha256"`
	RecordedAt string `json:"recorded_at"`
}
```

`candidate` 的规则覆盖 `cmd`、`internal`、`scripts`、`api`、`migrations` 五个目录和 `go.mod`、`go.sum`，它们存在才收；`Makefile` 必须存在。当前模板没有 `migrations` 与 `go.sum`，`1.0.0` 的候选实际由其余四个目录加 `go.mod`、`Makefile` 共 37 个文件构成。`inputs` 覆盖本版本 `需求.md`、`技术方案.md` 和跨版本共用的 `Specs/requirements/协议与数据.md`。两者都先对每个文件取 sha256，再按路径排序拼接后取一次 sha256，规则写在 `Snapshot` 的注释里。Loop Engineering 说的「证据绑定候选身份」就是这个形态：证据记的不是「通过」，是「在这份候选和这份输入上通过」。

幂等键防止重复登记变成重复状态。事件带 `--id` 时，同 id 同类型直接返回当前状态，同 id 不同类型以冲突拒绝。finding 同 id 给出相同 `--status` 时不改 severity 与 note，不写盘，revision 不变。`record`、`finding`、`event` 都在 `graph.json.lock` 独占锁内执行，锁被占用时以退出码 3 拒绝。

## 六个阶段与七类事件

阶段与事件的全部关系是一张小图，来自 `.ai/skills/spec-graph-workflow/SKILL.md`：

```text
planning --plan_confirmed--> implementing --implementation_done--> reviewing
reviewing --review_passed--> verifying --verified--> ready_to_deliver
reviewing --review_failed--> fixing --fix_done--> reviewing
ready_to_deliver / verifying --readiness_invalidated--> fixing
```

每条边的守卫定义在 `internal/specgraph/transition.go` 的 `transitions` 表里，只读三类输入：技术方案「交付状态」、绑定当前摘要的证据、finding 状态。守卫的共同形状是：推进到 `reviewing`、`verifying`、`ready_to_deliver` 这三个承载结论的阶段前，都要求对应 kind 的证据绑定当前摘要；`verified` 还要求所有 finding 已关闭、`user_acceptance` 为 `confirmed`。七类事件逐条的守卫条件见「Spec + Graph 落地」。

守卫拒绝时命令返回退出码 3 并说明缺什么，事件不写入，阶段不变。`reviewing` 与 `fixing` 之间可以循环多次，`1.0.0` 走了一轮：revision 14 `review_failed`，revision 26 `fix_done`，revision 41 `review_passed`。

## invalidated 是算出来的，不是记下来的

`graph.json` 里没有 `invalidated` 字段。`status` 每次运行都重新扫描输入与候选，用 `transition.go` 里的一个函数得出结果：

```go
// internal/specgraph/transition.go
func invalidated(st State, inputs, candidate Snapshot) bool {
	return contains(invalidatable, st.Stage) && (inputs.Digest != st.Inputs.Digest || candidate.Digest != st.Candidate.Digest)
}
```

失效只发生在 `reviewing`、`verifying`、`ready_to_deliver` 三个阶段，它们承载着已经得出的结论。`implementing` 和 `fixing` 本来就在改代码，摘要漂移是预期行为。`status` 还输出 `drifted_inputs` 与 `drifted_candidate`，指出漂移来自文档还是代码。

派生状态带来两个后果。谁都没法把 `invalidated` 改回 false，只能通过 `readiness_invalidated` 回到 `fixing` 重走实现、审查与验证，旧证据保留为历史但不再满足守卫。`readiness_invalidated` 自己也有守卫，readiness 没有失效时拒绝执行，挡住证据仍然有效时的人为退回；它只改阶段并追加事件，不删除任何证据或 finding。`check` 在 `ready_to_deliver` 且已失效时报告问题，提示执行这个事件。

`invalidated` 比较的是上次写入时绑定的摘要，守卫另外比较每条证据自带的摘要。两层叠起来，即使一次 `record` 让 `status` 恢复为 `invalidated: false`，旧证据仍然绑定旧摘要，`verified` 依旧不放行。

## 用户决定只住在「交付状态」块

`State` 里没有任何表示用户确认的字段。守卫需要用户决定时，调用 `internal/specdoc` 的 `ParseDeliveryStatus` 现读技术方案的「交付状态」块，取 `stage`、`user_acceptance`、`review` 三个值。`.ai/ai-rules.md` 的文件权限规则写明：`graph.json` 只能通过 `spec-graph` 命令写入，它记录执行阶段与证据身份，不记录用户决定。

两类信息不能互相干扰。`inputs` 中技术方案的摘要是剔除「## 交付状态」块之后的 sha256，剔除逻辑在 `internal/specgraph/digest.go` 的 `stripDeliveryBlock`。不剔，把 `user_acceptance` 从 `pending` 改成 `confirmed` 这个动作本身就会让 inputs 漂移，`verified` 永远过不去。

`1.0.0` 就停在这个位置。revision 46 记完最终门禁后，`go run ./cmd/spec-graph check 1.0.0` 退出码 0；接着执行 `event 1.0.0 verified`，被守卫拒绝，退出码 3：

```text
守卫不满足: verified: 技术方案「交付状态」user_acceptance 为 pending，需要 confirmed
```

当前 graph 为 `stage: verifying`、`revision: 46`、`invalidated: false`；技术方案「交付状态」为 `stage: verifying`、`user_acceptance: pending`、`review: pass`。Agent 能做的都做完了，版本仍未交付。用户确认后的顺序是：把 `user_acceptance` 改为 `confirmed`，执行 `event verified`，按 Step 7 回写文档并把 `stage` 改为 `delivered`，再运行 `make check` 与 `spec-graph check`。

## Graph 不运行命令，也不另存一份完成真相

`internal/specgraph` 的包注释写着「它只做确定性判定，不运行任何命令」。`record` 的 `--exit` 是 Controller 刚运行的命令的退出码，`--log` 指向该命令的输出文件，Graph 只保存退出码和日志的 sha256。`1.0.0` 的每条 `check` 证据背后都有一次真实的 `make check`。`review` 证据的退出码 1 与 0 对应 `spec-reviewer` 两轮的 `changes_required` 与 `pass`，由 Controller 登记；Subagent 的结论不直接写入，也不替代命令结果。

交付真相同样不在 Graph 里。`ready_to_deliver` 只表示守卫已放行，交付由「交付状态」的 `stage: delivered` 表示，`make check` 里的 `spec-check` 核对它。`spec-graph check` 核对 graph 与交付状态的一致性：交付状态为 `delivered` 而 graph 不在 `ready_to_deliver` 或已失效，报告问题；graph 到了 `ready_to_deliver` 而 `user_acceptance` 不是 `confirmed`，同样报告问题。它还报告残留的 `graph.json.lock`、存在 `graph.json` 但缺少 `技术方案.md`，以及无法计算摘要的情况。

## 边界：三个「不是」

| 它不是 | 依据 |
|---|---|
| 图数据库 | 只有一个 `graph.json`，用 `encoding/json` 读写；`store.go` 读取时按事件历史从 `planning` 重放一遍，重放结果与记录的 `stage` 不一致即拒绝；并发写入靠 `graph.json.lock` 与 `--expect-revision` |
| Agent runtime | 不创建、调度或等待任何 Subagent；委派由主会话按 Skill 执行，Codex 没有 Subagent 时可以只用 CLI 记录状态，实现与审查在同一会话串行完成 |
| 测试与门禁的替代 | 不产生任何行为证明；`make check`、`make test-integration`、`make smoke`、`api-verify` 仍是唯一的通过依据，Graph 只回答这些依据是否还绑定当前候选 |

## 与七步工作流的关系：叠加，不替换

`.ai/ai-rules.md` 把 Graph 列为可选扩展，在版本工作流里只出现两处：Step 4 确认方案后可以运行 `init` 与 `event plan_confirmed`，Step 6.2 用户确认后可以运行 `event verified`。不启用时，七个 Step、`spec-check` 和「交付状态」块照常工作。

启用后，Skill 的五个阶段落在原有 Step 上：

| Skill 阶段 | 对应 Step | 新增的动作 |
|---|---|---|
| A 初始化 | Step 4 | `init`、`event plan_confirmed` |
| B 实现 | Step 5 | 委派 `spec-implementer`，Controller 运行 `make check` 并 `record` |
| C 审查与修复 | Step 6.1 可选独立审查 | `finding` 登记、`record --kind review`、`review_failed` / `fix_done` / `review_passed` |
| D 验证与用户验收 | Step 6.1、6.2 | 各类门禁分别 `record`，用户确认后 `event verified` |
| E 交付与失效 | Step 7 及之后 | `spec-graph check`，变化后 `readiness_invalidated` |

Skill 的描述给出了适用范围：版本较大、需要独立审查、需要在需求或代码变化后判断哪些证据已失效。两个接口的 `1.0.0` 用它演示完整链路，日常小修改按「代码规范与日常修改」的流程走即可。

Graph Engineering 的分工到这里说得清了：Loop 让一个 Case 收敛，收敛结果是一条证据；Graph 让每条证据带着它的输入身份，输入一变就把「曾经通过」和「现在仍然有效」分开。`1.0.0` 的 46 次 revision，最后一步是 Agent 无法替用户完成的确认，Graph 能做的只是准确地停在那里。
