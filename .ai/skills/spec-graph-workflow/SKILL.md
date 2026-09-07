---
name: spec-graph-workflow
description: 可选工作流。把一个版本的开发编排为确定性状态图（spec-graph CLI 记录阶段、证据身份与 finding），由主会话作为唯一 Controller 推进，实现与审查分别委派给 Subagent spec-implementer 与 spec-reviewer。用于版本较大、需要独立审查、需要在需求或代码变化后判断哪些证据已失效的场景。
---

# /spec-graph-workflow — Spec + Graph 版本编排（可选）

> 用法：Claude Code `/spec-graph-workflow {version}`。Codex 没有 Subagent，可只使用 `spec-graph` CLI 记录状态，实现与审查在同一会话串行完成。
> 前提：已按 `.ai/ai-rules.md` 完成 Step 1–3，`Specs/requirements/{version}/需求.md` 存在。理论与边界见 `docs/spec-graph/理论.md`，命令细节见 `docs/spec-graph/落地.md`。

## 角色

| 角色 | 承担者 | 只做 |
| --- | --- | --- |
| Controller | 主会话 | 读需求与方案、切片、委派、作为唯一执行者运行 `make` 门禁与 `api-verify`、用 `spec-graph` 记录证据与推进阶段、回写文档、与用户交互 |
| Implementer | Subagent `spec-implementer` | 按切片测试先行写代码与测试，只跑 focused 测试，返回改动与 RED/GREEN 证据 |
| Reviewer | Subagent `spec-reviewer` | 只读三轮审查，返回 verdict 与 findings |
| 状态与证据 | `cmd/spec-graph` | 确定性阶段转换、守卫、证据身份与失效判定；不运行任何命令 |

用户决定（方案确认、验收确认、需求取舍类 finding）只记录在技术方案「交付状态」块，Graph 读取它作为守卫输入，不另存一份。

## 阶段与事件

```text
planning --plan_confirmed--> implementing --implementation_done--> reviewing
reviewing --review_passed--> verifying --verified--> ready_to_deliver
reviewing --review_failed--> fixing --fix_done--> reviewing
ready_to_deliver / verifying --readiness_invalidated--> fixing
```

每个事件的守卫读取三类输入：技术方案「交付状态」（用户决定）、`graph.json` 中绑定当前 candidate 与 inputs 摘要的证据、finding 状态。守卫不满足时命令以退出码 3 拒绝并说明缺什么；不要绕过，先补证据。

## 执行流程

### Step A — 初始化

1. Step 4 用户确认技术方案后，把「交付状态」`stage` 改为 `implementing`
2. `go run ./cmd/spec-graph init {version}`：创建 `Specs/technical/{version}/graph.json`（阶段 `planning`，记录当前 inputs 与 candidate 摘要）
3. `go run ./cmd/spec-graph event {version} plan_confirmed`：守卫读取「交付状态」`stage` 已不是 `planning`

### Step B — 实现（可并行切片）

1. 按技术方案「文件清单」与 Feature 切片；互不重叠的切片可同时委派多个 `spec-implementer`，每个给出：版本、Feature/AC、允许修改的路径、禁止路径
2. 收回每个 Implementer 的 YAML（改动文件、RED 观察、GREEN 命令、open_questions）；open_questions 属于需求歧义的先问用户
3. Controller 作为唯一执行者运行 `make check`，把结果记录为证据：
   `go run ./cmd/spec-graph record {version} --kind check --exit <退出码> --log <输出文件>`
4. `event {version} implementation_done`：守卫要求存在 `check` 证据、退出码 0、且绑定的 candidate / inputs 摘要与当前一致

### Step C — 审查与修复循环

1. 委派 `spec-reviewer`，收回 verdict 与 findings
2. 逐条登记：`finding {version} --id R1 --severity P1 --status open --note "..."`；随后由 Controller 判断 `--status accepted` 或 `--status rejected`（需求取舍类必须先问用户）
3. 记录审查证据：verdict `pass` 记 `record --kind review --exit 0`，`changes_required` 记 `--exit 1`
4. 有 open / accepted finding：`event {version} review_failed` 进入 `fixing`；修复按 6.3 先补失败测试再改代码（可再委派 Implementer），每条修好后 `finding --status fixed`，重跑 `make check` 并 `record`，然后 `event {version} fix_done` 回到 `reviewing`，再次委派审查；复审确认关闭的 finding 记 `--status verified`
5. 无阻塞 finding 且 review 证据为 0：`event {version} review_passed` 进入 `verifying`，同时把「交付状态」`review` 写成 `pass`

### Step D — 验证与用户验收

1. Controller 依次运行 `make test-integration`、`make test-race`（有并发改动）、`make smoke`、Skill `api-verify`，每项都 `record --kind <test-integration|test-race|smoke|api-verify> --exit <码> --log <文件>`；「交付状态」`stage` 改为 `verifying`
2. 按 ai-rules Step 6.2 进入用户验证；用户明确确认后把「交付状态」`user_acceptance` 改为 `confirmed`
3. `event {version} verified`：守卫要求 check / test-integration / smoke / api-verify 四类证据在当前 candidate 上退出码 0、所有 finding 为 verified 或 rejected、`user_acceptance` 为 `confirmed`

### Step E — 交付与失效

1. 按 Step 7 回写文档，「交付状态」`stage` 改为 `delivered`，运行 `make check`（含 `spec-check`）与 `go run ./cmd/spec-graph check {version}`
2. 之后任何需求、协议、技术方案或代码改动都会让 `status` 显示 `invalidated: true`，`check` 报告问题；此时执行 `event {version} readiness_invalidated` 回到 `fixing`，把「交付状态」`stage` 改回 `implementing`，按需求变更规则重新走 Step B–D，旧证据保留为历史但不再满足守卫

## 不要做的事

- 不手工编辑 `graph.json`；不用 `--expect-revision` 之外的方式处理并发写入
- 不让 Implementer 运行 broad 门禁或修改 `Specs/requirements/**`；不让 Reviewer 修改任何文件
- 不把 `record --exit 0` 当作凭空宣称：退出码必须来自 Controller 刚运行的命令，`--log` 指向其输出
- 不把 Graph 阶段 `ready_to_deliver` 当作交付：交付由「交付状态」`delivered` 表示，且两者必须一致（`check` 会核对）
