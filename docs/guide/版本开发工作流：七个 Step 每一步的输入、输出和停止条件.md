---
title: 版本开发工作流：七个 Step 每一步的输入、输出和停止条件
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - workflow
  - version-development
---

# 版本开发工作流：七个 Step 每一步的输入、输出和停止条件

用户说「开始 1.0.0 的开发」，Agent 不能直接去写 handler。`go-backend-template` 把一个版本的开发固定成七个 Step，顺序写在 `.ai/ai-rules.md` 的「AI 工作流（版本开发）」一节。`CLAUDE.md` 与 `AGENTS.md` 是指向它的符号链接，Claude Code 与 Codex 读到同一份（见「Rules 落地」）。

七这个数字不重要，三处硬停止才是形状：需求有歧义必须问，技术方案没确认不能编码，用户没做验收确认不能进 Step 7。前两处挡住 Agent 替用户做产品决定，第三处挡住 Agent 替用户宣布交付。前两处靠规则和技术方案里的固定位置兜住：「待确认问题」一行、「交付状态」的 `stage`。只有第三处由 `make check` 里的 `spec-check` 机器核对。

## 七个 Step 各自读什么、产出什么

| Step | 输入 | 产物 | 停止条件 |
| --- | --- | --- | --- |
| 1 读取上下文 | `Specs/technical/技术讲解.md`、`Specs/requirements/{version}/需求.md`、`Specs/requirements/协议与数据.md`、上一版本技术方案（如存在） | 对当前技术状态与本版本需求的完整读取 | 版本号不是 `x.y.z` 三段格式时先确认，不自行改写目录名 |
| 2 理解需求 | 每个功能的「做什么」「接口与数据」「业务规则与边界」「验收标准」 | 待确认问题清单 | 不明确、不完整或有歧义的需求必须向用户确认；新增第三方依赖要先取得同意 |
| 3 API 契约设计 | `协议与数据.md` 的通用约定、已有接口 | 每个接口的方法、路径、鉴权、请求体、成功响应、错误表，写入技术方案「API 契约」 | 无接口变更可跳过并说明；破坏性变更需升级路径前缀或经用户确认 |
| 4 出技术方案 | `make spec-init VERSION={version}` 生成的模版 | 填好的 `Specs/technical/{version}/技术方案.md`，「需求摘要」列出每个 `F-{version}-NNN` | 技术方案向用户展示并确认后才能编码；确认后「交付状态」`stage` 改 `implementing` |
| 5 写代码 | 已确认的技术方案与文件清单 | 失败测试、最小实现、单元 / 集成 / race 三层测试 | 每个业务规则和接口先写失败测试并确认失败原因正确 |
| 6 验证 | 当前代码与需求「验收标准」 | 命令输出摘要、接口核对表、验收标准与测试对应表 | 6.1 任一项失败不得进入 6.2；用户最终确认前 Step 6 不算完成，不得进入 Step 7 |
| 7 维护文档 | 实际实现 | 回写后的技术方案、技术讲解、Postman 集合、`scripts/smoke.sh`、README，`stage: delivered` | `spec-check` 拒绝 `user_acceptance` 未 `confirmed` 或 `review` 为 `pending` / `changes_required` 的 `delivered` |

Step 1 到 Step 4 一行代码都不产生，只读需求、只写技术方案。信息所有权在流程上就落成这个样子：`Specs/requirements/**` 由人维护，AI 禁止修改；`Specs/technical/**` 由 AI 维护，必须与代码一致（见「Spec 落地」）。需求有歧义就在 Step 2 问，不要自行假设。答复写进技术方案「需求摘要」下的「待确认问题」一行。

## 三处硬停止都有对应的文件字段

Step 2 的停止条件是内容判断。`ai-rules.md` 列了典型歧义：幂等语义、并发冲突处理、分页与排序、字段可空性、错误码归属。这一步没有机器门禁，只有「不得自行假设」这条规则和「待确认问题」这个固定位置。

Step 4 与 Step 6.2 的停止条件都落在技术方案末尾的「交付状态」块，机器可读的用户决定只有这一处。`internal/specdoc` 的 `ParseDeliveryStatus` 要求文档恰好有一个「## 交付状态」标题，其后第一个代码块是 `yaml` 围栏，只含三个键。`Validate` 执行的交付门禁是 `stage: delivered` 必须同时满足 `user_acceptance: confirmed`，且 `review` 为 `pass` 或 `not_required`。代码片段见「验证闭环」。

`stage` 的四个值 `planning | implementing | verifying | delivered` 逐一对应 Step：用户确认方案后改 `implementing`，进 Step 6.2 前改 `verifying`，Step 7 回写完成后改 `delivered`。`user_acceptance` 要等用户在 6.2 明确确认才能改 `confirmed`。Agent 跳过用户直接写 `delivered`，`make check` 会失败。「用户确认前不进 Step 7」在机器上就是这一条（见「验证闭环」）。

## Step 5 与 Step 6 内部也有顺序

Step 5 测试先行，三层测试都是必需交付物。单元测试不带 tag，`make test` 跑。集成测试首行 `//go:build integration`，`make test-integration` 跑，接数据库这类外部依赖时要用真实依赖覆盖成功与失败两条路径。碰 goroutine、锁、连接池就补 `make test-race`。Claude Code 下每次 `Edit` / `Write` 之后，`PostToolUse` Hook 运行 `scripts/check-format.sh` 做 gofmt 提醒。它是事后反馈，盖不住 Bash 产生的改动，也不替代 `make check`（见「Subagent 与 Hook 落地」）。

Step 6.1 是 Agent 自验，七项按顺序执行：`make check`（`go vet`、单元测试、gofmt 检查、`spec-check`）、`make test-integration`、`make test-race`（有并发改动时）、`make smoke`、Skill `api-verify` 对真实进程逐接口 curl 核对、对照需求「验收标准」逐条核对并确认每条都有测试固化，最后把 `stage` 改成 `verifying`。手动 curl 或 Postman 通过、却没有对应测试的行为，不算已验证。

独立审查是可选项，委派只读 Subagent `spec-reviewer`，返回 verdict 与 findings。`changes_required` 时，每条接受的 finding 先补失败测试再修，按 6.3 复验后再审一轮。审查结论不替代命令结果。做了审查就把 `review` 写成 `pending` / `changes_required` / `pass`，没做保持 `not_required`。

Step 6.2 是用户验证。Agent 展示验证结果，提示用户 `make run` 之后用 Postman 或 `api-verify` 报告里的 curl 命令验收；有 MCP 时 AI 可以代跑 `api/postman/` 集合。Postman 可选，用户确认必需。

Step 6.3 是反馈修复循环。用户反馈问题，先补一个能复现它的失败测试再改代码。逻辑改动重跑 `make check` 与相关集成测试，接口改动重跑 `api-verify` 对应接口和 `make smoke`，然后回到用户验证。用户反馈属于新增需求或需求变更的，不能当 bug 直接改。

## 需求变更不分大小，都要重新走确认

`ai-rules.md` 把「需求变更与新增规则」放在七个 Step 之前。开发过程中用户随时可能新增或修改需求，无论变更大小，AI 都要重新读需求文档找出变更点，更新技术方案的需求摘要、API 契约、模块设计、文件清单，向用户展示变更部分并确认后才能编码，编码完成后照样执行 Step 6 和 Step 7。「更新技术方案 -> 用户确认 -> 编码 -> 维护文档」这条链不能跳。

启用可选 Graph 后，这条规则有机器侧的对应物。`graph.json` 的 inputs 摘要覆盖需求、`协议与数据.md`，以及剔除「交付状态」块后的技术方案。阶段处于 `reviewing` / `verifying` / `ready_to_deliver` 时，任一处变化都会让 `status` 显示 `invalidated: true`。处于 `verifying` 或 `ready_to_deliver` 时执行 `event {version} readiness_invalidated`，回到 `fixing`，`stage` 改回 `implementing`，旧证据留作历史，不再满足守卫（见「Spec + Graph 落地」）。

## 日常代码修改不走版本流程，但不跳过测试与文档

bug 修复、小优化、配置调整不需要新版本目录，也不需要技术方案确认与用户验收。`ai-rules.md` 的「日常代码修改（非版本开发）」一节保留五条约束，和版本流程的差别集中在确认环节：

| 环节 | 版本流程 | 日常修改 |
| --- | --- | --- |
| 读取 | Step 1 四份文件 | `技术讲解.md`；涉及接口时再读 `协议与数据.md` |
| 方案 | `make spec-init` 生成技术方案并经用户确认 | 无 |
| 编码 | 按文件清单测试先行 | 先写复现问题的失败测试再修复 |
| 门禁 | 6.1 全部七项 | `make check`；触及外部依赖跑 `make test-integration`；接口行为变化再跑 `make smoke` 与 `api-verify` 对应接口 |
| 文档 | Step 7 全部回写并改 `delivered` | 改动影响技术全景时更新 `技术讲解.md`；涉及某版本功能时同步对应 `技术方案.md`；接口变化同步 Postman 集合 |
| 用户确认 | 方案确认与验收确认两次 | 无 |

两条路径用同一批命令、同一份技术讲解，差别只在用户要不要做两次确认。日常修改的边界见「代码规范与日常修改」。

## 1.0.0 演示：每个 Step 实际发生了什么

| Step | 实际发生 | 确认来源 |
| --- | --- | --- |
| 1 | 读取技术讲解、`Specs/requirements/1.0.0/需求.md`、`协议与数据.md`；`spec-check` 报告 `1 problem(s)`（缺少技术方案），`Specs/technical/` 下没有上一版本可读 | 无需确认 |
| 2 | 需求由 Agent 在用户授权下代写，落盘后按人工需求只读；技术方案「待确认问题：无」，长度按 Unicode 字符计数与 `content` 缺省为空两处歧义已在需求中写明 | 需求代写由用户授权替代 |
| 3 | 契约写入技术方案：`POST /v1/notes` 201 与三行 400 错误表，`GET /v1/notes/{id}` 200、404、405；请求体超 16 KiB 映射为 400 而非 413，因为 413 不在协议错误码表中 | 随方案一起确认 |
| 4 | `make spec-init VERSION=1.0.0` 生成技术方案并填写；`stage` 改 `implementing`；`spec-graph init` 与 `event 1.0.0 plan_confirmed` | 方案确认来自用户会话授权，记录在变更记录第一行 |
| 5 | `spec-implementer` 角色只跑 focused 测试，返回 11 个改动文件与每个用例的 RED/GREEN；主会话作为唯一 Controller 运行 broad 门禁 | 无需确认 |
| 6.1 | 四类门禁与 `api-verify` 通过；`spec-reviewer` 第一轮 `changes_required` 5 条，第二轮 `pass` 新增 R6；六条 finding 全部 `verified`；`stage` 改 `verifying`、`review` 改 `pass` | R1 严格满足需求「不得覆盖」，不放宽需求，不需要用户决定 |
| 6.2 | 未完成：`user_acceptance` 仍为 `pending` | 等待用户验收确认 |
| 7 | 未进入 | 依赖 6.2 |

Step 4 的 `spec-init` 实录见「spec-init」，含三次 `spec-check` 输出与两个负例的退出码。

Step 5 的角色扮演要如实说明：演示里是用通用 Agent 加载 `.claude/agents/spec-implementer.md` 正文来扮演实现者，不是 Claude Code 原生 `agents` 目录加载。Hook 也只做过脚本探针，真实会话中 `PostToolUse` 自动触发尚未记录。两条证据边界的细节见「Subagent 与 Hook 落地」。

Step 6.1 的审查修复循环最能说明「先补失败测试再修」。R1、R2 两条代码缺陷由 `spec-implementer` 角色先补失败测试再修，R3 到 R6 四条文档偏差由 Controller 回写。六条 finding 的内容与修复方式见「Subagent 与 Hook 落地」；`api-verify` 的 17 项场景与清理结果见「验证闭环」。

## Graph 记录了这条链，也在最后一步停住

`Specs/technical/1.0.0/graph.json` 把上面这条链记成 46 个 revision，两个候选摘要 `04a68378d8b4` 与 `2d03ec4f1c56`。逐 revision 的表见「Spec + Graph 落地」。

27 到 30 与 42 到 46 两组门禁证据绑定的 inputs 摘要不同。中间 R6 改过技术方案，而 `verified` 守卫要求 `check` / `test-integration` / `smoke` / `api-verify` 同时绑定当前 candidate 与当前 inputs，Controller 只能在同一候选上重新记录一遍。证据绑定候选身份在 Graph 里就长这样：Loop 管每条 finding 的 RED 到 GREEN，Graph 管旧结论对当前输入还成不成立（见「Spec + Graph 理论」）。

`go run ./cmd/spec-graph check 1.0.0` 退出码 0。接着执行 `event 1.0.0 verified`，守卫拒绝，退出码 3：

```text
守卫不满足: verified: 技术方案「交付状态」user_acceptance 为 pending，需要 confirmed
```

技术方案的「交付状态」块如下，`graph.json` 为 `stage: verifying`、`revision: 46`，`spec-graph status 1.0.0` 输出 `invalidated: false`：

```yaml
stage: verifying             # planning | implementing | verifying | delivered
user_acceptance: pending     # pending | confirmed
review: pass                 # not_required | pending | changes_required | pass
```

## 哪些确认被会话授权替代，哪些仍停在 pending

被替代的有两处，都留下了书面记录。需求代写：用户授权 Agent 撰写 `需求.md`，需求文件头部自述这一点，落盘后按人工需求只读。方案确认：技术方案写明「方案确认方式：用户在会话中授权本版本作为模板演示，视为已确认」，变更记录第一行同时记下 `stage` 改为 `implementing`。审查 finding 里没有需要用户决定的项。`spec-reviewer` 的交接合同规定，只有涉及需求取舍的 finding 才交给用户定；R1 选 `ErrAlreadyExists` 是把需求「不得丢失或覆盖」执行得更严格，不是取舍。

没有被替代的是用户验收。`user_acceptance` 仍为 `pending`，版本停在 `verifying`。确认后的动作顺序是：「交付状态」`user_acceptance` 改 `confirmed`，`event 1.0.0 verified` 进入 `ready_to_deliver`，Step 7 回写文档，`stage` 改 `delivered`，最后运行 `make check` 与 `go run ./cmd/spec-graph check 1.0.0` 核对两者一致。

1.0.0 同时成立两件事：`make check`、`make test-integration`、`make test-race`、`make smoke`（9 项检查）与 `api-verify`（17 项场景）全部通过，六条 finding 全部关闭；版本没有交付。绿色不等于交付。缺的不是命令，是用户在 Step 6.2 的那一句确认。
