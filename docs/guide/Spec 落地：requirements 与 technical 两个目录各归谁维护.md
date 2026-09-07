---
title: Spec 落地：requirements 与 technical 两个目录各归谁维护
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - spec
  - requirements
  - technical
  - delivery-status
---

# Spec 落地：requirements 与 technical 两个目录各归谁维护

`Specs/` 下只有两个目录。`requirements/` 由人维护，AI 只读；`technical/` 由 AI 维护，必须始终与代码一致。`.ai/ai-rules.md` 的目录树给它们标注 `[人 -> AI]` 和 `[AI -> AI]`，标的是信息流向，不是文档类型。一份文件能不能被 Agent 修改，先看它在哪个目录，不看它写了什么。

这条边界有机器守着。`ai-rules.md` 的「文件权限规则」把 `Specs/requirements/**` 列为 AI 禁止修改，需求有歧义时提问，不得替用户改需求；`Specs/technical/**` 列为 AI 负责维护，必须反映代码最新状态。`make check` 里的 `spec-check` 再核对两侧版本目录成对出现、需求中的每个 Feature ID 都出现在技术方案里、技术方案章节齐全、「交付状态」块合法（完整规则见「Command 落地：make 目标、脚本与 spec-check 各自守什么」）。产品行为的决定权在人，实现方式的记录责任在 AI，两份职责不共用一份文件。

## requirements：人写的三份文件各回答什么

| 文件 | 回答什么 | 什么时候被读 |
| --- | --- | --- |
| `Specs/requirements/需求模版.md` | 新版本需求的骨架，复制到 `{version}/需求.md` 后填写 | 人写需求时 |
| `Specs/requirements/协议与数据.md` | 对外接口的通用约定与对接方约束 | Step 1 必读，Step 3 出契约时必须遵守 |
| `Specs/requirements/{version}/需求.md` | 这个版本要做什么、怎样算做完 | Step 1 必读，Step 2 逐条理解 |

版本目录名必须与版本号完全一致，格式是语义化三段，如 `1.0.0`。`spec-check` 只承认 `x.y.z` 形式的目录名，`scripts/spec-init.sh` 收到 `v1` 这种版本号会以退出码 2 拒绝。用户给出的版本号不符合格式时，Agent 先确认，不自行改写目录名。

### 需求模版：一个功能写清四块，编号只有两种

模版把一个版本分成七节：版本信息、背景与目标、功能列表、数据变更、非功能需求、验证方式、变更记录。功能列表是主体，每个功能固定写「做什么」「接口与数据」「业务规则与边界」「验收标准」四块。模版头部的措辞很直接：写不清的地方 AI 会先提问，不会自行假设。

编号约定只有两种。功能编号 `F-{version}-NNN`，三位数字；验收标准 `AC-n`，在每个功能内部从 1 起编。技术方案的需求摘要与测试计划用它们追溯。`spec-check` 用正则 `F-([0-9]+\.[0-9]+\.[0-9]+)-[0-9]{3}` 从需求中抽取 ID，要求至少一个、版本段与目录名一致、每个都出现在技术方案里；技术方案引用其他版本的 ID 同样报错，实现在 `internal/speccheck/versions.go`。`AC-n` 不被机器核对，它的追溯落在技术方案测试计划表的「对应验收标准」列。

### 协议与数据：所有版本共用的接口合同

`协议与数据.md` 不属于任何版本，AI 出技术方案和写接口时都必须遵守。通用约定包括：HTTP/1.1，请求与响应均为 `application/json; charset=utf-8`；路由前缀 `/v1/`，破坏性变更升级前缀版本；路由用 `METHOD /path` 形式注册，方法不匹配返回 405；时间字段 RFC 3339 UTC；ID 是字符串；分页用 `?limit=&cursor=`，响应带 `next_cursor`。成功响应直接返回资源对象，不包裹 `data` 层；失败统一错误信封：

```json
{"error": {"code": "invalid_argument", "message": "email is required"}}
```

状态码表固定八行：400 `invalid_argument`、401 `unauthenticated`、403 `permission_denied`、404 `not_found`、409 `conflict`、429 `rate_limited`、500 `internal_error`、503 `unavailable`。健康检查 `GET /healthz` 返回 `{"status":"ok"}`，引入数据库等依赖后再新增 `GET /readyz`。鉴权和对接方约束两节留空，由业务填写。

## 1.0.0 的需求是怎样落在这套格式里的

`Specs/requirements/1.0.0/需求.md` 是模板自带的演示需求。它由用户授权 Agent 代写，文件头部写明落盘后按人工需求只读处理；后续所有 Step 都按这条办，实现与审查阶段没有登记过对它的修改。两个功能如下：

| 功能 | 接口 | 验收标准 |
| --- | --- | --- |
| `F-1.0.0-001` 创建笔记 | `POST /v1/notes` | AC-1 到 AC-5：201 与四个字段、`id` 匹配 `^n_[0-9a-f]{12}$`；title 缺失 / 空白 / 101 字符 400；content 2001 字符 400；非法 JSON 与未知字段 400；100 个并发创建 ID 各不相同 |
| `F-1.0.0-002` 读取笔记 | `GET /v1/notes/{id}` | AC-1 到 AC-3：创建后读取 200 且内容相同；不存在的 ID 404 `not_found`；`DELETE /v1/notes/{id}` 405 |

验收标准原文写成可以直接变成断言的句子，字段名、状态码、错误码和边界值都在句子里：

```markdown
- [ ] AC-1 合法请求返回 201，响应含 `id`、`title`、`content`、`created_at` 四个字段，`id` 匹配 `^n_[0-9a-f]{12}$`
- [ ] AC-2 缺少 `title`、`title` 为空白、`title` 为 101 个字符时均返回 400 `invalid_argument`，`error.message` 含 `title`
```

业务规则一节另外写明 title 按 Unicode 字符计数、`content` 缺省为空字符串、并发创建「不得丢失或覆盖」。`spec-reviewer` 的 R1 finding 就是拿「不得覆盖」这句对照 `memstore.Store.Save` 的覆盖语义提出来的。

## technical：一份全景、一份模版、每版一份方案

| 文件 | 内容 | 谁在什么时候改 |
| --- | --- | --- |
| `Specs/technical/技术讲解.md` | 项目技术全景 | AI 在 Step 7 和日常修改后更新 |
| `Specs/technical/技术方案模版.md` | 版本方案骨架 | `make spec-init VERSION=x.y.z` 用 `sed` 替换 `{version}` 生成版本方案 |
| `Specs/technical/{version}/技术方案.md` | 这个版本怎样实现，含「交付状态」块 | AI 在 Step 4 填写并交用户确认，Step 7 按实际实现回写 |
| `Specs/technical/{version}/graph.json` | 可选，`spec-graph` 的阶段与证据身份 | 只能通过 `spec-graph` 命令写入，不手工编辑 |

`spec-init` 的前提是人工需求已存在。1.0.0 的实录：只创建 `Specs/requirements/1.0.0/需求.md` 时，`make spec-check` 输出 `spec-check: 1 problem(s)`，提示缺少技术方案并给出 `make spec-init VERSION=1.0.0`；运行后输出 `created: Specs/technical/1.0.0/技术方案.md`；在需求摘要中引用 `F-1.0.0-001` 与 `F-1.0.0-002` 后，`spec-check ok (1 version(s))`。需求不存在时脚本以退出码 1 结束，不会替人补一份（详见「spec-init」）。

### 技术方案模版：十一节里有八节被机器核对

模版共十一节：需求摘要、影响面分析、API 契约、模块设计、数据模型与迁移、配置变更、测试计划、风险与回滚、文件清单、交付状态、变更记录。`spec-check` 规则 7 从里面挑出八个二级标题，要求各恰好出现一次：需求摘要、API 契约、模块设计、测试计划、风险与回滚、文件清单、交付状态、变更记录，名单在 `internal/speccheck/versions.go` 的 `planHeadings`。影响面分析、数据模型与迁移、配置变更这三节按需填写，机器不核对。

各节怎么写，看 1.0.0 的方案。需求摘要一行一个 Feature，覆盖列取模版给出的「是 / 部分 / 否（原因）」之一，1.0.0 两行均为「是」。待确认问题写「无」，确认方式注明是用户在会话中授权本版本作为模板演示，指向变更记录；变更记录首行写明方案确认来自用户对「实现 1.0.0 演示」的会话授权。

API 契约是实现层面的最终定义。`POST /v1/notes` 的错误表把三种 400 的 `message` 都写死：`invalid request body`、`title must be 1-100 characters`、`content must be at most 2000 characters`。`GET /v1/notes/{id}` 的 404 message 固定为 `note not found`。api-verify 的 17 项场景里，两条笔记接口的场景按这张表逐条核对，另有 healthz 与 ping 两项来自 `技术讲解.md` 的接口清单。模块设计一行一个包，`internal/memstore` 那行写明 `Save` 对已存在 ID 返回 `note.ErrAlreadyExists`，不覆盖。

测试计划表在编码前写场景，编码后回写为实际文件与用例名。1.0.0 的表头写作「用例（实际落地）」：

```markdown
| 单元（存储） | `internal/memstore/store_test.go` | `TestStore_SaveFind_RoundTrip`、`TestStore_Find_Missing`、`TestStore_ConcurrentSave`（100 goroutine）、`TestStore_Save_DuplicateID`（不覆盖，返回 `ErrAlreadyExists`） | F-1.0.0-001 AC-5 |
```

「对应验收标准」这一列是 `AC-n` 唯一的追溯位置。R4 的一半与 R6 都指向它。R4 指出没有回写实际用例名，另一半是模块设计缺 logger 字段；R6 指出有一个用例名挂错了行。两处都由 Controller 回写修正。

## 交付状态块：三个键怎样把用户决定和命令结果分开

技术方案倒数第二节「交付状态」，一段说明引文之后是一个 yaml 围栏。`Specs/technical/技术方案模版.md` 给出初始值：

```yaml
stage: planning              # planning | implementing | verifying | delivered
user_acceptance: pending     # pending | confirmed
review: not_required         # not_required | pending | changes_required | pass
```

`internal/specdoc/delivery.go` 的 `ParseDeliveryStatus` 负责读它。文档中恰好一个 `## 交付状态` 标题，围栏代码块内的不算；标题之后第一个代码块必须是 ```` ```yaml ````，中途遇到下一个标题就报错；块内只允许 `stage`、`user_acceptance`、`review` 三个键，不重复、不缺失、围栏必须闭合。按行切分不用 `bufio.Scanner`，超长行不会被静默截断。`Validate` 校验三个枚举，然后执行唯一一条门禁：`stage: delivered` 要求 `user_acceptance` 为 `confirmed`，且 `review` 为 `pass` 或 `not_required`；代码片段见「验证闭环」。

三个键由 AI 写入，每次取值变化对应 `ai-rules.md` 工作流里的一个 Step 编号：Step 4 确认方案后 `stage: implementing`，Step 6.1 通过后 `stage: verifying`，做了审查才改 `review`，Step 6.2 用户确认后 `user_acceptance: confirmed`，Step 7 回写后 `stage: delivered`。逐项的触发者与 1.0.0 实际值见「spec-init」。

`user_acceptance` 记录的是用户的决定，AI 不能替用户填 `confirmed`。`技术讲解.md` 把这块称为唯一机器可读的用户决定记录；`graph.json` 只记录执行阶段与证据身份，不记录用户决定。

1.0.0 停在这里：`stage: verifying`、`user_acceptance: pending`、`review: pass`。`make check`、`make test-integration`、`make test-race`、`make smoke` 全部通过，`spec-reviewer` 第二轮 verdict pass，api-verify 17 项场景通过。版本仍未交付。启用 `spec-graph` 的演示里，`go run ./cmd/spec-graph check 1.0.0` 退出码 0，`event 1.0.0 verified` 却被守卫拒绝：「守卫不满足: verified: 技术方案「交付状态」user_acceptance 为 pending，需要 confirmed」，退出码 3。命令结果绑定候选身份记在证据里，用户确认单独占一个键，谁也不能用前者冒充后者。用户确认后的动作顺序固定：改 `confirmed` -> `event verified` -> Step 7 回写 -> `stage: delivered` -> `make check` 与 `spec-graph check`（见「验证闭环」、「Spec + Graph 落地」）。

## 技术讲解.md 回答的是「现在的代码是什么样」

`技术讲解.md` 是 Step 1 第一个读的文件，日常修改前也先读它。它与版本方案的分界：方案记录一个版本的决定、契约和测试证据，讲解回答当前代码整体是什么样。`spec-check` 规则 3 要求它含四个二级标题：目录结构、接口清单、测试策略、已知问题与待优化项，名单在 `internal/speccheck/speccheck.go` 的 `overviewHeadings`。当前文件另有项目概述、核心模块、关键设计约定三节，目录结构里每个文件后面跟一句职责说明，接口清单列出 `GET /healthz`、`GET /v1/ping`、`POST /v1/notes`、`GET /v1/notes/{id}` 四条路由。

「始终与代码一致」有过失效记录。1.0.0 审查的 R5 finding 就是技术讲解与 README 落后于代码，第二轮审查前由 Controller 回写。Step 7 与日常修改流程都把更新这份文件列为固定动作。

## 需求变了怎么办，实现和需求冲突了又怎么办

`ai-rules.md` 的「需求变更与新增规则」不分变更大小，固定四步：重新读取需求文档并对比找出变更点；更新技术方案的需求摘要、API 契约、模块设计、文件清单；向用户展示变更部分并确认后才能编码；编码完成后同样执行 Step 6 和 Step 7。Step 6.3 第 5 条补了一句：用户反馈属于新增需求或需求变更时，按这条规则处理，不得当作 bug 直接改。变更的起点是人改 `requirements/`，AI 负责把变化传导到 `technical/` 和代码。

实现与需求冲突时不改需求。R1 finding 指出 `memstore.Store.Save` 遇到 ID 冲突会静默覆盖，与需求「不得覆盖」相悖。处理方式是让代码严格满足需求：`Save` 返回 `note.ErrAlreadyExists`，先新增 `TestStore_Save_DuplicateID` 与 `TestService_Create_SaveAlreadyExists` 取得 RED，再修实现取得 GREEN。这个修法没有放宽需求，不需要用户决定。如果修法要放宽需求，就必须回到用户那里，由人改 `requirements/`，再走一遍上面四步。

一个新会话接手时，两个目录各答一半：这个版本要做什么、验收项是哪几条，看 `requirements/`；实现成了什么样、测试落在哪些文件、用户有没有确认，看 `technical/`。两边答案不一致时，先按所有权决定该改哪一边，再动代码。
