---
title: 初始化与资产同步 Skills
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - concept
  - spec-coding
---

# 初始化与资产同步 Skills

## 概念

一套模板只有被真正用起来才有价值，用起来意味着三条路径：从模板复制出一个新工程、把已有工程改造成同一套规范、模板更新后把改动同步回各个工程。

这三件事都是多步骤、跨文件、容易漏的操作。复制出新工程要改 module 路径、改服务名、重建符号链接；存量工程接入要补 Specs 目录、补行为准则、补门禁依赖的工具包；同步要区分「模板提供的」和「工程自己的」，前者可以覆盖，后者不能碰。靠人记步骤，第三次就会漏掉一项，而漏掉的那项通常在几周后才以「为什么这个工程没有 spec-check」的形式暴露。

模板把这三条路径各自固化成一个 Skill：`init-project`、`spec-coding-init`、`sync-ai-assets`。每个 Skill 写明步骤编号、每步动哪些文件、哪些文件受保护、结束时用什么命令验证。

## 本工程怎么落地

### init-project：从模板到新工程

`.ai/skills/init-project/SKILL.md` 分五步（Step 0 至 Step 4），交互规则是每次只问一个配置项、不提供默认值、用户回复 `跳过` 时保留占位，首次运行的必填项不接受跳过。

Step 0 读当前目录 `go.mod`。module 为 `example.com/go-backend-template` 说明站在模板里，走 Step 1；其他 module 说明已是业务工程，跳过 Step 1 直接补信息；没有 `go.mod` 则停止。同时扫描占位符：`.ai/ai-rules.md` 的「项目简介」是否仍含「模板工程」、`README.md` 标题、Postman 集合的 `info.name`、`CLAUDE.md` / `AGENTS.md` 链接是否断裂。

Step 1 调 `.ai/skills/init-project/rename_module.sh`，脚本自己打印五步进度：

| 步骤 | 动作 |
| --- | --- |
| [1/5] 复制模板 | `tar` 复制以保留符号链接，排除 `.git`、`bin`、`.claude/settings.local.json`、`.DS_Store` |
| [2/5] 替换 module 路径 | `*.go`、`go.mod`、`*.md`、`*.sh`、`*.json`、`Makefile`，跳过 `./.ai/skills/init-project/*` |
| [3/5] 替换模板名称 | `*.md`、`*.json`、`Makefile`、`smoke.sh`，跳过整个 `./.ai/skills/*`；顺带重命名 Postman 集合文件 |
| [4/5] 检查符号链接 | `CLAUDE.md`、`AGENTS.md` → `.ai/ai-rules.md`，`.claude/skills`、`.agents/skills` → `../.ai/skills` |
| [5/5] 验证构建与测试 | `go build ./...` 与 `go test ./...`；没有 `go` 命令只发警告并提示手动跑 `make check` |

脚本开头 `set -euo pipefail`，六种情况以退出码 1 挡在前面：参数不足两个、目录名不符合 `^[A-Za-z_][A-Za-z0-9_-]*$`、新 module 等于 `example.com/go-backend-template`、module 路径含非法字符、源目录 `go.mod` 不是模板、目标目录已存在。它还区分 BSD sed 与 GNU sed 的 `-i` 语法，用 `sedi` 函数包一层。两处替换都把正则元字符转义后再替。

Step 2 收集信息，服务名称和一句话描述是必填，默认监听端口、主要外部依赖、鉴权方式可选。Step 3 用 Edit 精确替换写进 `.ai/ai-rules.md`、`Specs/technical/技术讲解.md`、`README.md`、Postman 集合，端口还要同步 `internal/config/config.go` 与 `config_test.go`。鉴权方式是个例外——Skill 只提示用户自己去改 `Specs/requirements/协议与数据.md`，AI 不动这个文件。

Step 4 收尾：首次运行且无 `.git` 时 `git init` 并提交 `chore: initialize project from go-backend-template`；确认双 Agent 结构齐全；跑 `make check`，`spec-check` 应输出 `spec-check ok (0 version(s))`。

### spec-coding-init：存量工程接入

面向已经在跑的 Go 工程，交互规则里有一条硬约束：改造过程不修改任何业务代码，只新增文档、AI 资产与符号链接。

Step 0 确定 `PROJECT_ROOT`，判断是否已改造（`Specs/` 与 `.ai/ai-rules.md` 都在就转去用 `sync-ai-assets`），再按 `.claude/` / `CLAUDE.md` 与 `.codex/` / `.agents/` / `AGENTS.md` 的存在情况判断 AI 平台，两者皆无时默认双 Agent。这一步还检查嵌套 `.git`：`git status` 出现 `new file mode 160000` 要先 `git reset HEAD -- <path>`。

Step 1 建 Specs 四件套。原则一句话：模版文件以模板工程为准，业务内容以本项目为准。

| 文件 | 本项目已存在 |
| --- | --- |
| `requirements/需求模版.md` | 用模板版本覆盖 |
| `technical/技术方案模版.md` | 用模板版本覆盖 |
| `requirements/协议与数据.md` | 保留 |
| `technical/技术讲解.md` | 保留，Step 6 更新 |
| `requirements/{version}/需求.md` | 保留，不存在也不创建 |

Step 2 同步基础设施，含旧结构迁移（`.claude/commands/{name}.md` 移到 `.ai/skills/{name}/SKILL.md`，普通文件形态的 `CLAUDE.md` 内容并入 `.ai/ai-rules.md` 后改成符号链接）。`cmd/spec-check/`、`internal/speccheck/`、`internal/specdoc/` 必须复制，因为 `make check` 依赖它们，复制后要改 import 路径；`cmd/spec-graph/` 一系是可选扩展，问过用户才动。Makefile 要具备 `check`、`spec-check`、`spec-init`、`test-integration`、`test-race`、`smoke` 六个目标。

Step 3 是两阶段扫描。Stage A 做结构发现：`go.mod` 的直接依赖、`go list ./...` 包清单、`cmd/*/main.go` 的初始化顺序、路由（同时搜 `HandleFunc(`、`.GET(`、`Route(`、`Group(` 等写法，覆盖 net/http 与常见框架）、中间件链、配置项与默认值、存储与迁移工具、外部调用，最后记录 `go vet` 与 `go test -cover` 的基线但不修复。

Stage B 才按 Stage A 的结果挑目录深入，`internal/httpapi/` 看路由表与错误映射，`repository/` 看端口实现与事务边界，`migrations/` 看迁移顺序与最新 schema。分两阶段是因为存量工程的目录名不可预测，先看全貌再决定读哪些包，比按写死的路径去猜准得多。这一步还顺带识别废弃代码：无调用方的导出函数、注释掉的大段代码、缺 `_test.go` 的核心包，全部记进技术讲解的「已知问题」，不自动删。

Step 5 生成行为准则时，已存在 `.ai/ai-rules.md` 的情况下新增章节插入、已有章节只补充，禁止直接覆盖。Step 6 分三轮读代码生成技术讲解，生成后对比 `go list ./...` 确认每个包都被覆盖。Step 7 验证结构、验证基础设施、确认 `make check` 与改造前结果一致。

### sync-ai-assets：模板更新后回流

Step 1 定位模板源，参数是 `.git` 结尾或 `git@` / `http` 开头时 `git clone --depth 1` 到临时目录，随后用 `grep -q 'go-backend-template' "$SRC/go.mod"` 验明身份。

Step 2 分六类对比：Skills、`.ai/ai-rules.md`、Specs 模版、配置与脚本、符号链接、工具包与说明文档。每一类都写死了对本地内容的处置。

Skills 逐目录比，本项目自有的标「仅本地」不动；带 `SOURCE.md` 的第三方 Skill 按其中的版本号比较，模板更新时整目录替换，不做逐文件合并。行为准则按 `##` 拆章节比，项目简介、目录结构、当前公共实现清单三处默认保留本项目内容，只提示不改。`协议与数据.md` 明确不同步。`.claude/settings.json` 只合并 `permissions.allow` 与 `hooks.PostToolUse`，其余键保留；`spec-reviewer` 的 `tools` 必须保持 `Read, Grep, Glob`。`cmd/spec-check/`、`internal/speccheck/`、`internal/specdoc/` 视为模板工具代码，整目录以模板为准。

Step 3 输出变更摘要，每项带 `[无变化]` / `[更新]` / `[新增]` / `[仅本地]` / `[保留]` / `[待定]` / `[已跳过]` 标签。Step 4 等用户确认才执行 Step 5，冲突项逐一展示差异再问。Step 6 删掉临时克隆目录并跑 `make check`，结果与同步前一致才算完成。

## 扩展思路

**把自己的脚手架步骤加进 init-project。** 业务工程往往还要建 CI 变量、注册服务名、申请数据库、配置日志上报。这些步骤加成 `init-project` 的 Step 2 询问项与 Step 3 更新目标即可，格式和现有表格一样：一列询问项、一列更新目标。要保持「每次只问一个」和「不提供默认值」，新增项默认走可选，不轻易升级成必填。

**把模板放进内部仓库当同步源。** `sync-ai-assets` 的 Step 1 已经支持 git 地址，企业内把模板推到内部 Git 服务，各业务工程就能用同一条 `/sync-ai-assets <内部仓库地址>` 拉齐资产，不必依赖谁本地有一份模板副本。模板侧对应要维护一个稳定分支，避免把半成品同步出去。

**把同步做成定期任务。** `sync-ai-assets` 的 Step 2 和 Step 3 是只读的，跑完只产出一份变更摘要，可以放进定时任务每周跑一次，把摘要发给负责人。真正的写入在 Step 4 的用户确认之后，定期任务只负责让人知道有哪些待同步项。

**明确存量接入时不该自动改的东西。** 现有清单是业务代码、`Specs/requirements/协议与数据.md`、人工需求文件、本项目自有的 Skill 与行为准则章节、已按接口改写过的 `scripts/smoke.sh`。工程接手后如果有别的人工维护资产——运行手册、告警规则、灰度配置——同样要在 Skill 里点名加入保留清单，否则某次同步就会把它们抹掉。判断标准是这份文件的最终解释权在模板还是在工程。
