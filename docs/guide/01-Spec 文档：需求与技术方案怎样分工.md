---
title: Spec 文档：需求与技术方案怎样分工
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - concept
  - spec-coding
---

# Spec 文档：需求与技术方案怎样分工

## 概念

Spec 要回答三个问题：谁能改需求、谁维护方案、完成由什么判定。这三条定死了，多写的文档才有意义。

所有权。需求是人对 AI 的输入，只能人写。AI 读到含糊的地方要提问，不能顺手把需求改成自己方便实现的样子——需求一旦被实现方改写，验收就失去了独立参照。技术方案反过来，由 AI 维护，必须跟着代码走，记录的是代码的当前状态。

可核对性。「做完了」不能靠一句话宣布。需求里每个功能有编号，技术方案要逐个引用；验收标准要能一条条转成测试用例；交付状态得是机器能读、能拒绝的字段。

## 本工程怎么落地

`Specs/` 下两个目录，权限相反：

- `Specs/requirements/` — 人工维护，AI 只读。`.ai/ai-rules.md` 的「文件权限规则」写死 `Specs/requirements/**` 禁止 AI 修改。
- `Specs/technical/` — AI 维护，必须反映代码最新状态。

四份文件各管一段：

| 文件 | 归属 | 作用 |
| --- | --- | --- |
| `Specs/requirements/需求模版.md` | 人 | 新版本需求从这里复制；固定「做什么 / 接口与数据 / 业务规则与边界 / 验收标准」四段结构 |
| `Specs/requirements/协议与数据.md` | 人 | 跨版本的接口约定：路由前缀 `/v1/`、错误信封、状态码与 `error.code` 对照表、对接方约束 |
| `Specs/technical/技术方案模版.md` | AI | 单个版本的实现方案骨架，含机器可读的「交付状态」块 |
| `Specs/technical/技术讲解.md` | AI | 项目技术全景：目录结构、核心模块、接口清单、测试策略、已知问题 |

版本需求落在 `Specs/requirements/{version}/需求.md`，技术方案落在 `Specs/technical/{version}/技术方案.md`，两侧目录名都是 `x.y.z`。

### 编号

功能编号 `F-{version}-NNN`，三位数字；验收标准编号 `AC-n`。需求写 `F-1.0.0-001`，技术方案的「需求摘要」表里必须出现同一个 ID，测试计划的「对应验收标准」列写到 `F-1.0.0-001 AC-1`。编号是追溯链的锚点，`internal/speccheck/versions.go` 里的 `featureIDs` 用正则 `F-([0-9]+\.[0-9]+\.[0-9]+)-[0-9]{3}` 提取，并排除紧贴字母数字的前缀和四位以上的编号。

### 交付状态

技术方案末尾的「## 交付状态」是唯一机器可读的完成判定：

```yaml
stage: planning              # planning | implementing | verifying | delivered
user_acceptance: pending     # pending | confirmed
review: not_required         # not_required | pending | changes_required | pass
```

`internal/specdoc/delivery.go` 的 `ParseDeliveryStatus` 要求全文恰好一个「## 交付状态」标题（围栏代码块内的不算），其后第一个代码块必须是 ` ```yaml `，块内只允许这三个键，重复键、未知键、缺键、未闭合都报错。`Validate` 再执行门禁：`stage: delivered` 要求 `user_acceptance` 是 `confirmed`，且 `review` 是 `pass` 或 `not_required`。审查挂着 `changes_required` 就写不成 delivered。

三个键各自记录不同来源、互不推导的判断：`stage` 是执行阶段，`user_acceptance` 是用户在 Step 6.2 的明确确认，`review` 是独立审查结论。

### make spec-init

`scripts/spec-init.sh` 只做一件事：把 `技术方案模版.md` 里的 `{version}` 替换成版本号，写到 `Specs/technical/{version}/技术方案.md`。前提缺一不可：

- 版本号匹配 `^[0-9]+\.[0-9]+\.[0-9]+$`，否则退出码 2
- `Specs/requirements/{version}/需求.md` 已存在，否则退出码 1 并提示「先由人工从 需求模版.md 复制并填写」
- `Specs/technical/技术方案模版.md` 存在，否则退出码 1
- 目标技术方案不存在，已存在时退出码 1 不覆盖

AI 不能替用户创建需求，这条约束是脚本层面的，不靠提示语。

### spec-check 里与 Spec 相关的规则

`make check` 包含 `go run ./cmd/spec-check`，十条规则中直接管 Spec 的是：

- 规则 1：需求模版、协议与数据、技术讲解、技术方案模版、`.ai/ai-rules.md`、`Makefile` 存在且为普通文件
- 规则 5：两侧版本目录名匹配 `x.y.z`；requirements 有版本必须有对应技术方案，反之亦然
- 规则 6：需求里至少一个 Feature ID，版本段与目录一致，每个都出现在技术方案；技术方案不得引用其他版本的 ID
- 规则 7：技术方案的八个二级标题（需求摘要、API 契约、模块设计、测试计划、风险与回滚、文件清单、交付状态、变更记录）各恰好出现一次
- 规则 8：交付状态块能被 `specdoc` 解析并通过门禁
- 规则 10：版本目录内只允许 `需求.md` / `技术方案.md`、`graph.json` 和点文件

退出码 0 表示无问题并打印 `spec-check ok (N version(s))`，1 表示有问题并逐行输出 `路径: 问题`，2 表示参数或 I/O 错误。

## 扩展思路

**把约束沉到 requirements。** 评审意见、数据契约、对接方要求的字段命名与签名规则，都属于人给 AI 的输入，写进 `协议与数据.md` 的「对接方约束」或版本需求的「业务规则与边界」，不要只在对话里说。写在这两处，AI 每个版本 Step 1 都会重新读；写在对话里，换个会话就没了。

**技术方案加自己团队要的小节。** 模版的八个必需标题是 spec-check 的下限，不是上限——加「灰度与开关」「监控指标」「容量评估」不会触发规则 7，因为它只校验这八个标题各出现一次。加进 `技术方案模版.md`，之后每个版本 `make spec-init` 都会带上。

**把更多约定写成检查项。** 目前十条规则覆盖的是文件、标题、链接、编号、交付状态、Postman 集合。团队自己的硬约束——迁移文件必须成对出现回滚、每个新路由必须进冒烟脚本、破坏性变更必须写升级说明——都可以在 `internal/speccheck/` 加规则，用同样的 `c.add(path, format, args...)` 汇报，`make check` 自动带上。规则要配正反例测试，否则只是多一条会误报的检查。

**需求来自需求管理系统。** 需求在 Jira、飞书或别的系统里时，`Specs/requirements/{version}/需求.md` 是同步下来的快照，同步动作由人执行，AI 仍然只读。同步时保留 `F-{version}-NNN` 与 `AC-n` 编号，追溯链和 spec-check 规则 6 才继续成立。反过来让 AI 直接拉取和回写需求系统，实现方就能改动自己的验收依据，只读边界不再成立。
