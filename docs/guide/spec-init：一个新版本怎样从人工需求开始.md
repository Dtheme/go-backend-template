---
title: spec-init：一个新版本怎样从人工需求开始
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - spec-init
  - version
  - feature-id
  - spec-check
---

# spec-init：一个新版本怎样从人工需求开始

新版本的第一件产物是人工写的 `Specs/requirements/{version}/需求.md`，代码和技术方案都排在它后面。`make spec-init VERSION=x.y.z` 只在这份需求存在之后才生成技术方案骨架。从这一步起，版本号、Feature ID 与「交付状态」块进入 `make check` 里 `spec-check` 每次都要核对的范围。

`scripts/spec-init.sh` 只有 27 行，把版本目录、需求编号与技术方案绑在一起。后面的 Step、审查和可选的 Graph 都从这个绑定出发。

## spec-init 先拒绝，后生成

命令只做一件事：把 `Specs/technical/技术方案模版.md` 里的 `{version}` 替换成给定版本号，写入 `Specs/technical/{version}/技术方案.md`。落盘之前有五处拒绝：版本号格式两条检查、需求不存在、模版缺失、方案已存在。Makefile 还拦下空 `VERSION`，合计六种失败输入，命中任何一种都不写文件。

```sh
# scripts/spec-init.sh
case "$V" in
    *[!0-9.]*|"") echo "用法: make spec-init VERSION=x.y.z（语义化三段版本号）" >&2; exit 2 ;;
esac
echo "$V" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$' || { echo "版本号必须是 x.y.z 形式: $V" >&2; exit 2; }

[ -f "$REQ" ] || { echo "人工需求不存在: $REQ（先由人工从 需求模版.md 复制并填写）" >&2; exit 1; }
[ -f "$TEMPLATE" ] || { echo "缺少模版: $TEMPLATE" >&2; exit 1; }
[ -e "$PLAN" ] && { echo "已存在，不覆盖: $PLAN" >&2; exit 1; }
```

下表是在模板临时副本上逐条触发的真实结果：

| 输入 | 输出 | 退出码 |
| --- | --- | --- |
| `make spec-init`，未传 `VERSION` | Makefile 先拦截：`用法: make spec-init VERSION=x.y.z` | 2 |
| `VERSION=v1` | `用法: make spec-init VERSION=x.y.z（语义化三段版本号）` | 2 |
| `VERSION=1.0` | `版本号必须是 x.y.z 形式: 1.0` | 2 |
| `VERSION=9.9.9`，需求不存在 | `人工需求不存在: Specs/requirements/9.9.9/需求.md（先由人工从 需求模版.md 复制并填写）` | 1 |
| `VERSION=1.0.0`，技术方案已存在 | `已存在，不覆盖: Specs/technical/1.0.0/技术方案.md` | 1 |
| `VERSION=1.0.0`，需求存在且方案不存在 | `created: Specs/technical/1.0.0/技术方案.md` | 0 |

需求不存在时脚本不替人补一份。`.ai/ai-rules.md`「文件权限规则」规定 `Specs/requirements/**` 禁止 AI 修改，这条规则在命令层就是这样兑现的：需求的信息所有权在人工，工具连创建都不做。不覆盖管的是另一件事，已经填写过的方案不会被再次初始化抹掉。脚本不碰其他文件。一个版本默认只有需求与技术方案两份版本文件；启用 Graph 时 technical 侧多一份 `graph.json`，由 `spec-graph init` 创建，谁维护哪份见「Spec 落地：requirements 与 technical 两个目录各归谁维护」。

成功时第二行输出写明下一步：`按模版各节填写技术方案，交用户确认后再编码；make check 会运行 spec-check 核对 Feature ID 与交付状态。`

## 版本号同时是目录名、Feature ID 前缀和 {prev} 的排序键

四个地方共用同一个 `x.y.z` 字符串。

- `Specs/requirements/{version}/` 与 `Specs/technical/{version}/` 的目录名与版本号完全一致。`internal/speccheck/versions.go` 用 `^[0-9]+\.[0-9]+\.[0-9]+$` 判断目录，不合法的目录名报「版本目录名必须是 x.y.z 形式」，不参与后续检查。
- Feature ID 固定为 `F-{version}-NNN`，版本段必须等于所在目录名，否则报「Feature ID … 的版本与目录 … 不一致」。技术方案引用其他版本的 ID 同样报错。
- `{prev}` 由 ai-rules Step 1 定义：按语义版本排序后小于当前版本的最大目录。Agent 读取 `Specs/technical/{prev}/技术方案.md` 了解最近变更；用户给出的版本号不符合格式时先确认，不自行改写目录名。
- `cmd/spec-graph` 的 `version` 参数用同一条正则校验，不满足时以用法错误退出。

版本目录里只允许固定文件：requirements 侧只有 `需求.md`，technical 侧只有 `技术方案.md` 与 `graph.json`，点文件跳过，其余一律报「意外文件」。technical 侧有版本目录而 requirements 侧没有，则报「没有对应人工需求」。

## Feature ID 与 AC 编号是贯穿全程的追溯键

验收标准在需求里写下时就带两个编号。后面每份产物用编号回指它，不靠措辞相似度匹配。

| 产物 | 编号出现的位置 | 由谁写 | 1.0.0 实例 |
| --- | --- | --- | --- |
| `Specs/requirements/{version}/需求.md` | `### F-{version}-NNN` 标题；验收标准列表 `AC-n` | 人工 | `F-1.0.0-001` 创建笔记 AC-1 到 AC-5；`F-1.0.0-002` 读取笔记 AC-1 到 AC-3 |
| 技术方案「需求摘要」 | 表格首列，必须列出需求中的每个 Feature ID | AI，Step 4 | 两行，「本方案覆盖」均为「是」 |
| 技术方案「测试计划」 | 「对应验收标准」列写 `F-{version}-NNN AC-n` | AI，Step 4 写计划，Step 7 回写实际用例名 | `internal/httpapi/notes_test.go` 一行对应 `F-1.0.0-001 AC-1 至 AC-5；F-1.0.0-002 AC-1 至 AC-3` |
| `spec-implementer` 交接 YAML | `scope: F-{version}-NNN / AC-n` | Subagent 返回 | 委派按 Feature 与 AC 切片，只跑 focused 测试 |
| `api-verify` 报告 | 「对应测试」列写用例名，找不到记「缺测试」 | Agent，Step 6.1 | 真实进程 17 项场景全部通过 |

`api-verify` 报告没有单独的 Feature ID 列。用例名连回测试计划，测试计划的「对应验收标准」列再连回 AC，链条是需求 AC、测试计划用例、真实进程场景，三段都能回查。「每条验收项对应 Case」在本模板里就是这个形状。契约冻结见「API 契约设计：接口在编码前怎样被冻结」，测试分层见「测试分层：单元、集成、race、冒烟各自证明什么」。

`speccheck` 识别 ID 有两条边界：`F` 前一个字符是 ASCII 字母或数字（如 `REF-1.0.0-001`，整体不算 ID）和超过三位的编号（`F-1.0.0-0002`）都不算，避免把引用样例误当声明。需求至少要有一个 ID，否则报「至少一个 Feature ID（F-1.0.0-NNN）」。

## spec-check 在一个版本里的三次典型结果

`spec-check` 在版本的每个阶段都能跑，结果跟着文档状态变，不是交付前才跑一次的终检。下面三段输出来自把模板复制到临时目录后重放 1.0.0。

第一次，只有 `Specs/requirements/1.0.0/需求.md`：

```text
$ go run ./cmd/spec-check
Specs/technical/1.0.0/技术方案.md: 缺少技术方案，运行 make spec-init VERSION=1.0.0
spec-check: 1 problem(s)
```

第二次，`make spec-init VERSION=1.0.0` 之后没有做任何填写：

```text
$ go run ./cmd/spec-check
Specs/technical/1.0.0/技术方案.md: 技术方案未引用 F-1.0.0-002
spec-check: 1 problem(s)
```

只报 `-002` 的原因：模版的需求摘要与测试计划各有一处 `F-{version}-001` 占位，`sed` 替换后已经是 `F-1.0.0-001`，规则只发现 `-002` 缺失。占位符能凑齐第一个 ID，凑不齐第二个。需求有多少个 Feature，方案就必须逐个写到。

第三次，需求摘要填入两个 Feature ID：

```text
$ go run ./cmd/spec-check
spec-check ok (1 version(s))
```

三次退出码分别是 1、1、0。`make check` 把 `spec-check` 与 `go vet`、`go test ./...`、`gofmt -l` 串在一起，前两种状态下整个 `make check` 失败。模板自身跑 `make check`，spec-check 输出的是最后这一行。

## 交付状态块的每一次改动都有指定的触发者

三个键的每次取值变化都对应 ai-rules 里的一个 Step 编号。改动来源只有两类：用户的明确决定，Agent 在特定 Step 的回写。`spec-init` 生成时的初值如下：

```yaml
stage: planning              # planning | implementing | verifying | delivered
user_acceptance: pending     # pending | confirmed
review: not_required         # not_required | pending | changes_required | pass
```

| 改动 | 触发者 | 依据 | 1.0.0 实际 |
| --- | --- | --- | --- |
| `stage: planning` | `spec-init` 写入模版初值 | 模版 | 创建后即为此值 |
| `stage: implementing` | 用户确认技术方案后由 Agent 改 | Step 4 第 3 条 | 确认来自用户的会话授权，记入技术方案变更记录 |
| `review: pending / changes_required / pass` | 委派 `spec-reviewer` 时由 Agent 按 verdict 写；不做审查保持 `not_required` | Step 6.1 可选独立审查 | 第一轮 `changes_required`（R1 至 R5），第二轮 `pass` |
| `stage: verifying` | 自验通过、进入 6.2 前由 Agent 改 | Step 6.1 第 7 条 | 当前值 |
| `user_acceptance: confirmed` | 只有用户明确确认后才改 | Step 6.2 第 4 条 | 仍为 `pending` |
| `stage: delivered` | Step 7 文档回写完成后由 Agent 改 | Step 7 第 7 条 | 未发生 |

`internal/specdoc` 的 `ParseDeliveryStatus` 要求整个文档恰好一个 `## 交付状态` 标题，围栏代码块内的不算；标题后第一个代码块必须是 yaml，且只含这三个键。`Validate` 先校验三个键都在允许的枚举内，再只对 `stage: delivered` 加两条门禁：`user_acceptance` 必须是 `confirmed`，`review` 必须是 `pass` 或 `not_required`。在 1.0.0 的当前状态下把 `stage` 提前改成 `delivered`，真实结果是：

```text
Specs/technical/1.0.0/技术方案.md: stage 为 delivered 但 user_acceptance 不是 confirmed
spec-check: 1 problem(s)
```

文档层的「绿色不等于交付」就落在这两条门禁上。`make check`、`make test-integration`、`make test-race`、`make smoke` 与 `api-verify` 全部通过，审查也已 `pass`，Agent 仍然不能自己把版本写成交付。缺的是用户在 6.2 的决定，见「验证闭环：自验通过之后为什么还要等用户确认」。

## 启用 Graph 时，init 与 plan_confirmed 也在这一步发生

`spec-graph` 是可选的。一旦启用，它的前两条记录正好落在 `spec-init` 之后、编码之前：Step 4 用户确认方案并把 `stage` 改为 `implementing` 后，依次执行 `go run ./cmd/spec-graph init 1.0.0` 与 `event 1.0.0 plan_confirmed`。

`init` 创建 `Specs/technical/1.0.0/graph.json`，revision 0、阶段 `planning`，记下 inputs 与 candidate 两份摘要。inputs 固定为三份文件：`Specs/requirements/1.0.0/需求.md`、`Specs/requirements/协议与数据.md`、`Specs/technical/1.0.0/技术方案.md`，其中技术方案先剔除「交付状态」块再算 sha256，用户决定的变化不算需求或方案漂移。candidate 覆盖 `cmd`、`internal`、`scripts`、`api`、`migrations` 下的普通文件与 `go.mod`、`go.sum`、`Makefile`。

`plan_confirmed` 的守卫只读一件事：技术方案「交付状态」的 `stage` 不再是 `planning`，否则以退出码 3 拒绝并提示「技术方案「交付状态」stage 仍为 planning」。1.0.0 的 `graph.json` 里，这条事件是 `evt-1`，revision 1，`planning -> implementing`。

此后 45 次 revision 记录了 check、四类门禁、六条 finding 与审查事件，当前停在 stage `verifying`、revision 46、`invalidated: false`。在这个位置执行 `event 1.0.0 verified`，证据绑定与 finding 条件都已满足，卡在守卫的最后一条：

```text
守卫不满足: verified: 技术方案「交付状态」user_acceptance 为 pending，需要 confirmed
```

退出码 3。Graph 没有另存一份「已验收」，它读的仍是 `spec-init` 生成的那个 yaml 块。这个块既是 `spec-check` 的门禁对象，也是每个 Graph 守卫的用户决定来源。两者从同一份技术方案读事实，分工见「Spec + Graph 理论：Loop 管局部收敛，Graph 管结论是否仍然有效」与「Spec + Graph 落地：spec-graph CLI、graph.json 与 Subagent 编排」。

接手一个版本时核对三件事：目录名、Feature ID 版本段与 `VERSION` 参数是否是同一个 `x.y.z`；需求里的每个 `F-{version}-NNN` 是否都出现在技术方案里；交付状态块的当前值对应哪个 Step、下一次改动该由谁触发。三件事都能用 `make spec-check` 与 `go run ./cmd/spec-graph status {version}` 直接核对，不依赖会话记忆。
