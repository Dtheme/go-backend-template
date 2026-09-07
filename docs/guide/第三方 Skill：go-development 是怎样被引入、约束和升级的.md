---
title: 第三方 Skill：go-development 是怎样被引入、约束和升级的
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - third-party-skill
  - go-development
  - license
---

# 第三方 Skill：go-development 是怎样被引入、约束和升级的

`go-backend-template` 的 `.ai/skills/` 下放着六个 Skill。五个自研（见「Skill 落地」），一个来自 netresearch，叫 `go-development`。它是目录里唯一带 `SOURCE.md` 的 Skill，也是唯一一个 AI 被禁止修改的 Skill。

复制文件是最简单的一步。真正要回答的是三件事：内容与本工程规则冲突时听谁的，它附带的检查要不要算进门禁，上游更新时怎样替换而不留下本地补丁。`SOURCE.md`、`.ai/ai-rules.md` 的文件权限规则、`sync-ai-assets`，各答一件。在 `ai-rules.md` 的定位里，这份 Skill 只是参考资料，规则单源仍然是 `ai-rules.md`。

## 为什么引入而不是自己重写一份

`go-development` 1.15.1 的 `references/` 目录有 20 份文档，主题包括测试分层、race 常见坑、slog、API 防御、fuzz、契约与不变量测试、依赖升级。都是通用 Go 工程实践，不含本工程的业务判断。重写一遍，得到的是一份内容相近、没有上游维护的副本。

模板必须自己写的是与工作流绑定的那部分：`ai-rules.md` 里的七个 Step、`spec-check` 的规则、`api-verify` 怎么验证。上游不提供这些，也不该由上游决定。分工由此固定：规则来自 `ai-rules.md`，通用实践按需从 `go-development` 读取，冲突以前者为准。`ai-rules.md` 在 Step 5 写明了加载时机，编码时按需加载 `references/testing.md`、`architecture.md`、`api-design.md`、`logging.md`；「代码规范 / 测试」一节把 `fuzz-testing.md`、`contracts-and-invariants.md` 列为扩展阅读。加载由规则文件指定，不靠 Skill 自己的触发词。

## SOURCE.md 里记了什么

`.ai/skills/go-development/SOURCE.md` 是模板在该目录里唯一新增的文件，其余 26 个文件与上游一致。它记录的内容：

| 项目 | 记录值 |
|---|---|
| 来源 | `https://github.com/netresearch/go-development-skill` |
| 版本 | 1.15.1（`plugin.json` / `SKILL.md` frontmatter） |
| 引入方式 | 从发布包 `go-development-skill-main` 复制 `skills/go-development/` 全部内容与两份许可证 |
| 许可证 | MIT（代码与脚本）AND CC-BY-SA-4.0（文档），原文为同目录 `LICENSE-MIT`、`LICENSE-CC-BY-SA-4.0` |
| 本地改动 | 无，仅新增 `SOURCE.md` |
| 脚本副作用 | `scripts/verify-go-project.sh` 只读检查（`go vet` 与文件存在性），不修改文件、不联网 |
| 完整性摘要 | 26 个上游文件的 SHA-256 列表 |

最有约束力的是完整性摘要。它由 `shasum -a 256` 对复制进来的全部文件生成，下游工程用同一条命令重算，就能知道目录有没有被动过。列表的前三行：

```text
4b34c2a0857a7b24ddd66ba7449c756c5de4cc2ad528744624099ecc3731b52e  LICENSE-CC-BY-SA-4.0
6c29a3abb028408de1bd94979fa3150b8a540712517d9c6909b264e402f1f3b4  LICENSE-MIT
901fcc34f7181e51bbb6853eea0c047273b1b91ff69b69c031ca76b202e2831e  SKILL.md
```

在模板上重算，26 行输出与 `SOURCE.md` 记录逐行一致；`SOURCE.md` 开头为 `SKILL.md` 单独列出的摘要，与列表中该文件的那一行相同。「这个第三方目录没被改过」，这句话绑定的是这 26 个摘要，不是引入记录，也不是谁的记忆。

`SOURCE.md` 还把 20 份 references 按本模板的用途分成四组：8 份适用（编码与测试时按需加载）、5 份可选（`linting.md`、`makefile.md`、`mutation-testing.md`、`single-build-release.md`、`lefthook-template.md`）、4 份仅当业务引入对应能力时参考（cron、resilience、docker、ldap）、3 份与本模板无关（`branch-protection.md`、`reusable-workflows.md`、`awesome-go-submission.md`）。frontmatter 里的 `compatibility: "Requires go 1.21+, golangci-lint, docker."`，以及「For reviews, invoke security-audit / enterprise-readiness / github-project」，都被标为不强制、未引入、忽略。

## 五条已知冲突，一律以 ai-rules.md 为准

上游 `SKILL.md` 只有 92 行，其中几条与本模板的取舍直接相反。`SOURCE.md` 逐条列出并给出处理：

| 上游内容 | 本模板规则 | 处理 |
|---|---|---|
| Consistency：`Config precedence: defaults < config file < env vars < flags` | 配置只来自环境变量，无配置文件与 flag | 忽略上游层级，只保留 defaults < env vars |
| Testing：`Always use t.Parallel()` | 只在无共享可变状态且有明显收益时用，默认串行 | 按需，不强制 |
| References 段首行的 lefthook 安装提示（`ls lefthook.yml ...`） | 不使用 git hook，门禁走 Makefile | 读到该行不执行、不提示安装 lefthook |
| Quality Gates 的 `golangci-lint`、`staticcheck`、`govulncheck` | `make lint` / `make vuln` 可选，`make check` 不依赖 | 已安装则作为加强项运行 |
| frontmatter `description` 触发词（cron、Docker API、LDAP、golangci-lint） | 加载时机由 Step 5 指定 | 不依赖自动触发 |

五条都不是理论分歧，1.0.0 演示里条条有落点。`internal/config/config.go` 只用 `os.Getenv` 读 `HTTP_ADDR` 与 `SHUTDOWN_TIMEOUT`，没有配置文件层。`internal/memstore/store.go` 用 `sync.RWMutex` 保护共享 map，`store_test.go` 的 `TestStore_ConcurrentSave` 启动 100 个 goroutine 并发写入。全仓没有一处 `t.Parallel()`，共享状态的正确性由 `make test-race` 证明，门禁里它与 `make check`、`make test-integration`、`make smoke` 一起通过。`make lint` 与 `make vuln` 因 golangci-lint、govulncheck 未安装按设计跳过，`Makefile` 里这两个目标都先用 `command -v` 探测，再决定执行还是打印跳过提示（片段见「MCP 落地」）。

仓库里没有 `lefthook.yml`，也没有 `.golangci.yml`。

「以 ai-rules.md 为准」有个前提：冲突得先被写出来。规则文件里只写一句「冲突时以本文件为准」，Agent 读到上游那句 `Always use t.Parallel()`，还是要自己判断这算不算冲突。`SOURCE.md` 把判断提前做完，会话里只剩查表。

## checkpoints.yaml 与 evals 不接入门禁

上游附带两份可执行的检查资产。`checkpoints.yaml` 有 GD-01 到 GD-26 共 26 个编号，分 `mechanical` 与 `llm_reviews` 两组，后者是 GD-20、GD-21、GD-22 三条 review 提示。`evals/evals.json` 有 22 条用例。两者原样保留，不接入 `make check`，也不接入任何 Step。

多条与本模板的取舍直接冲突：

| 编号 | 上游要求 | 本模板现状 |
|---|---|---|
| GD-02（severity: error） | `go.sum` 必须存在 | 仅标准库，`go.sum` 不生成 |
| GD-05 | `.golangci.yml` 应存在 | 没有，lint 可选 |
| GD-21 | review 提示把 `t.Parallel()` 列为检查项 | 默认串行 |
| GD-23 | Makefile 应有 `all:` 目标 | 没有，入口是 `check` |
| GD-24 | 测试目标应带 `-coverprofile` | 不写只为覆盖率的断言，不生成覆盖率文件 |
| GD-26 | `lefthook.yml` 应存在 | 不使用 git hook |

GD-02 的严重级别是 error。接进门禁的后果很具体：一个 `make check`、`make test-integration`、`make test-race`、`make smoke` 全部通过的候选，会因为一个本来就不该存在的文件被判失败。evals 里的 `add_ldap_integration`、`setup_cron_scheduler`、`docker_client_integration`、`integration_test_docker`、`setup_lefthook`、`awesome_go_submission`，断言的能力本模板一个都没有。`SOURCE.md` 的结论是：需要时挑单条手动执行，不作为交付判断。

有一处记录留待下次升级顺手校正。`SOURCE.md` 把 `checkpoints.yaml` 概括成「26 条机械检查加 3 条 LLM review」，按文件编号逐条数，是 23 条 `mechanical` 加 3 条 `llm_reviews`，编号总数才是 26。结论不受影响，数字要与文件一致。

## 禁止修改、停用与升级

`ai-rules.md` 的「文件权限规则」用一个可机器识别的特征区分两类 Skill：

```markdown
- `.ai/skills/{自研 Skill}/**` — AI 可按真实需要修改，改动要在 Skill 内保持步骤自洽。
- `.ai/skills/{第三方 Skill}/**`（目录内含 `SOURCE.md`）— **AI 禁止修改**，只能整目录升级并更新 `SOURCE.md`。
```

`.claude/agents/spec-implementer.md` 把 `.ai/skills/go-development/**` 与 `Specs/requirements/**`、`graph.json` 并列为始终禁止修改的路径。1.0.0 演示里，实现由加载了这份正文的通用 Agent 完成，不是 Claude Code 原生 agents 目录加载，主会话作为唯一 Controller 运行 broad 门禁。运行时工具限制没有端到端记录，目录未被改动的证据来自摘要重算，不来自 Subagent 的自述（见「Subagent 与 Hook 落地」）。

禁止修改，不代表上游内容不可质疑。理由在下一次升级：有了本地补丁，升级就变成三方合并。要调整的地方一律写进 `SOURCE.md` 的「处理」列，上游文件保持原样，SHA-256 列表才有意义。

停用与升级各只有一条路径：

- 停用：删除 `.ai/skills/go-development/`，并从 `ai-rules.md` 的「Skills」表移除 `go-development` 行。引用该目录的位置不止这一处：规则文件里 Skills 表下方那段来源说明（写明来自 netresearch、许可证与 `SOURCE.md` 位置）、Step 5 与「代码规范 / 测试」两处按需加载的指引、`README.md` 的 Skills 表（`ai-rules.md` 要求 Skills 变化时同步 README）、`Specs/technical/技术讲解.md` 的目录结构、`spec-coding-init` Step 7 的结构验证清单、`spec-implementer.md` 与 `docs/spec-graph/落地.md` 的禁改路径，停用时一并清理，否则会留下指向不存在目录的指引。
- 升级：下载新版本后整目录替换，更新 `SOURCE.md` 的版本、SHA-256 与日期；本地不做修改。

`sync-ai-assets` 把同一规则带到业务工程。它在 2.1 对比 Skills 时，对目录内有 `SOURCE.md` 的第三方 Skill 按版本号比较，模板版本更新就整目录替换，不做逐文件合并；逐文件展示 diff 只针对自研 Skill。同步后 Step 6 要求 `make check` 结果与同步前一致。

`init-project` 的 `rename_module.sh` 用 tar 整体复制模板，替换模板名称时排除 `./.ai/skills/*`；替换 module 路径那一步只排除 `init-project`，仍会扫描 `go-development/` 下的 `.md`、`.sh`、`.json`。上游文件目前不含 `example.com/go-backend-template`，新工程拿到的目录与模板逐字节相同，`shasum -a 256` 重算即可确认。上游升级后要重新核对这一点。

`spec-graph` 的 candidate 摘要覆盖 `cmd`、`internal`、`scripts`、`api`、`migrations`、`go.mod`、`go.sum`、`Makefile`，不包含 `.ai/skills`。升级这份 Skill 不会让已记录的门禁证据失效，与它不参与门禁的定位对得上。

还有一层边界容易混淆。上游 `SKILL.md` 的 `allowed-tools` 声明了 `Bash(docker:*)` 与 `Bash(golangci-lint:*)`，`.claude/settings.json` 的 8 条 `permissions.allow` 是 go、gofmt、make、curl、git、bash、lsof、shasum，没有这两项。前者是上游对自己的预授权声明，后者才是本模板的权限配置。引入 Skill 没有改动 `settings.json`。Skill 在真实会话中是否按这行扩大预授权，没有端到端记录，模板的文件权限规则也不依赖它。

## 引入下一个第三方 Skill 时核对什么

把上面的做法抽成清单，每一项都能在本模板里找到对应落点：

| 检查项 | 本模板的落点 |
|---|---|
| 先回答为什么不自研 | 通用 Go 实践不含业务判断，自研只得到无上游维护的副本 |
| 固定来源、版本、引入日期与方式 | `SOURCE.md` 开头的字段列表 |
| 许可证原文随目录入库 | `LICENSE-MIT`、`LICENSE-CC-BY-SA-4.0`，声明为 `MIT AND CC-BY-SA-4.0` |
| 全目录 SHA-256，且给出可重算的命令 | 26 行摘要，`shasum -a 256` |
| 本地零改动，调整只写在记录文件 | 「本地改动：无」，处理写在两张表里 |
| 审计脚本副作用 | `verify-go-project.sh` 只读、不联网 |
| 逐条列出与本地规则的冲突并定优先级 | 五条冲突，一律以 `ai-rules.md` 为准 |
| 上游自带的检查是否接入门禁，写明理由 | `checkpoints.yaml`、`evals` 不接入，GD-02 等 6 条冲突 |
| 预授权声明与实际权限配置分开看 | `allowed-tools` 与 `settings.json` 的 8 条 allow |
| 模板内所有引用点 | `ai-rules.md` 的 Step 5、「代码规范 / 测试」、Skills 表及其下方的来源说明段落、文件权限规则；`README.md` Skills 表、`技术讲解.md` 目录结构、`spec-coding-init` 结构验证清单、`spec-implementer` 与 `docs/spec-graph/落地.md` 禁改路径 |
| 停用与升级路径，同步工具按版本整目录替换 | `SOURCE.md` 末节，`sync-ai-assets` 2.1 |
| 复制与同步后门禁结果不变 | `rename_module.sh` 末步的构建与测试，`sync-ai-assets` Step 6 的 `make check` |

清单里没有「审查上游内容是否正确」。上游文档的质量由上游负责。模板负责的是四件事：哪些适用、哪些冲突、改动能否被发现、替换能否不留痕迹。四点都满足的第三方 Skill，就可以和自研 Skill 放在同一个目录里，通过 `.claude/skills` 与 `.agents/skills` 两个指向 `../.ai/skills` 的符号链接，被 Claude Code 与 Codex 读到。
