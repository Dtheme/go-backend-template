---
title: Skill：怎么用，怎么沉淀自己的
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - concept
  - spec-coding
---

# Skill：怎么用，怎么沉淀自己的

## 概念

Skill 是一个文件，里面写着一类重复工作该怎么一步步做。它针对的问题很具体：同一件事，这次做了七步，下次做了四步，漏掉的那三步每次还不一样。把步骤、停止条件和收尾写进文件，下次照着执行。

它和另外三样东西经常被混在一起：

| 机制 | 形态 | 生效方式 | 例子 |
| --- | --- | --- | --- |
| 行为准则 | 一份长期规则文档 | 每次会话都在上下文里，约束所有动作 | `.ai/ai-rules.md` 的文件权限规则、七个 Step |
| Skill | 一段带编号的流程 | 被显式调用时才展开，用完即走 | `/api-verify` 的启动、逐接口核对、清理 |
| Subagent | 独立上下文里的一个角色 | 主会话委派，带自己的工具集，返回结构化结果 | `spec-reviewer`（tools 只有 `Read, Grep, Glob`） |
| Hook | 挂在固定事件上的一条命令 | 事件触发就跑，不经过模型判断 | `PostToolUse` 匹配 `Edit\|Write` 时执行 `scripts/check-format.sh` |

准则说「不能改 `Specs/requirements/**`」，Skill 说「先读 `go.mod` 判断当前状态，再问新目录名」。前者一直有效，后者只在做那件事时有效。Subagent 换的是上下文和工具边界，Hook 换的是触发时机——它不看模型想不想跑。

Skill 不是权限边界。它写着「必须先跑 `make check`」，但拦不住任何人跳过；真正的门禁是 `make check` 里的 `go vet`、单元测试、gofmt 和 `spec-check` 这几条命令的退出码。

Skill 也不重复准则。`.ai/ai-rules.md` 已经写了版本开发的七个 Step、文件权限规则和验证命令表，Skill 只接管其中步骤多、跨文件、容易漏项的那几个点位，剩下的引用过去。两边写同一件事，改一处忘一处的问题很快就会出现。

## 本工程怎么落地

每个 Skill 是一个目录：`.ai/skills/{name}/SKILL.md`。五个自研 Skill 的 frontmatter 只有 `name` 和 `description` 两个字段，正文从 `# /{name} — 用途` 起，往下是用法一行、可选的「交互规则」、按 Step 编号的执行流程；第三方 Skill 保持上游自己的格式，字段更多，标题也不按这个来。`.claude/skills` 和 `.agents/skills` 是两条指向 `../.ai/skills` 的符号链接，Claude Code 敲 `/{name}`、Codex 敲 `${name}`，读的是同一份文件；`CLAUDE.md`、`AGENTS.md` 链到 `.ai/ai-rules.md` 也是这一招。

Skill 需要的支持文件放在同目录，跟 `SKILL.md` 一起被复制走。自研 Skill 里只有 `.ai/skills/init-project/rename_module.sh` 一个：`SKILL.md` 的 Step 1 用 `bash "$TEMPLATE_ROOT/.ai/skills/init-project/rename_module.sh" "$NEW_DIR_NAME" "$NEW_MODULE" ...` 调它，文件级重命名的实际逻辑在脚本里，Skill 只负责问参数和解释输出。

两个要向用户提问的 Skill 共用一组交互规则：每次只问一个配置项，等回复再问下一个；用户回 `跳过` 就保留占位继续下一项（`init-project` 首次运行的必填项不接受跳过，`spec-coding-init` 可改用代码分析结果推断）；不提供默认值或建议值。第三个带交互规则的 `sync-ai-assets` 换了一套：先展示变更摘要，等用户确认才执行，冲突项逐一展示差异再问，本项目自有的 Skill、章节、配置一律保留不删。

五个自研 Skill 各自钉在生命周期的一个时刻：

| Skill | 时刻 | 入口 |
| --- | --- | --- |
| `init-project` | 模板目录变成一个业务工程 | `/init-project` |
| `spec-coding-init` | 已有 Go 工程接入 Spec 工作流 | `/spec-coding-init [项目目录路径] [--template 模板工程路径]` |
| `sync-ai-assets` | 模板更新后回流到业务工程 | `/sync-ai-assets [模板工程路径]` |
| `api-verify` | Step 6.1 自验的最后一道契约核对 | `/api-verify [version]` |
| `spec-graph-workflow` | 可选：版本大、要独立审查、要判断哪些证据已失效 | `/spec-graph-workflow {version}` |

它们的形状是一致的：先判状态，再执行，最后出报告。`init-project` 的 Step 0 读 `go.mod`，module 还是 `example.com/go-backend-template` 就走复制重命名，换过了就跳过 Step 1 直接补信息；Step 4 收尾要确认符号链接、`make check` 里 `spec-check` 输出 `spec-check ok (0 version(s))`，再列出仍为占位的项。`sync-ai-assets` 的 Step 2 分成 Skills、行为准则、Specs 模版、配置与脚本、符号链接、工具包六类逐一对比，Step 6 清理临时 clone 目录并跑 `make check` 确认同步没有影响构建。

`spec-coding-init` 面对的是存量工程，所以扫描依据必须是项目实际结构而不是写死的路径。它的 Step 3 分两阶段：Stage A 做结构发现，读 `go.mod`、跑 `go list ./...`、找入口 `main`、搜路由注册（`HandleFunc(`、`.GET(`、`Route(` 等覆盖多种框架）、搜配置读取与存储驱动；Stage B 再按 Stage A 的结果动态选模块深入。Step 6 逆向生成 `技术讲解.md` 时读三轮——核心骨架、业务模块、工具与外部调用——生成后对着 `go list ./...` 确认每个包都被覆盖到。整个过程不修改任何业务代码，只新增文档、AI 资产与符号链接。

`api-verify` 是这里最完整的一份，可以当模板看：前置校验六条（`go version` 与 `curl --version` 可用，`make check` 与 `make test-integration` 已通过，Postman MCP 可选，读契约来源，读 `协议与数据.md` 的错误信封，确认外部依赖就绪），Step 1 用 `API_VERIFY_PORT:-18090` 起服务并写进 `mktemp -d` 出来的独立目录（与 `scripts/smoke.sh` 的 18080 错开），Step 2 每个接口至少跑成功路径、每种错误场景、方法不匹配三类，Step 3.5 反过来核对每个场景在 `*_test.go` 里有没有对应用例，Step 4 无论成败都 kill 进程、`rm -rf "$RUN_DIR"`、用 `lsof -i :$PORT` 确认端口释放，Step 6 规定失败了怎么办——先补失败测试再修代码，只重跑失败接口，契约本身不合理则不得私自改契约。

`spec-graph-workflow` 是唯一带确定性状态的一个：阶段转换和证据身份交给 `cmd/spec-graph`，守卫不满足时命令以退出码 3 拒绝并说明缺什么，Skill 正文只写谁在什么阶段做什么，末尾一节「不要做的事」明确禁止手工编辑 `graph.json`、禁止把 `record --exit 0` 当作凭空宣称。

`.ai/ai-rules.md` 的文件权限规则给 Skill 划了两条线：自研 Skill 目录 AI 可按真实需要修改，改动要保持步骤自洽；第三方 Skill 目录（内含 `SOURCE.md`）AI 禁止修改，只能整目录升级并更新 `SOURCE.md`。这条线在 `sync-ai-assets` 的 Step 2.1 落成了两种比较方式：自研 Skill 逐文件对比，标成新增、更新、无变化或仅本地；第三方 Skill 直接按 `SOURCE.md` 里的版本号比，模板版本更新就整目录替换，不做逐文件合并。合并一个上游目录的代价远高于换掉它。

## 扩展思路

先按模板把一个真实版本从需求跑到交付，别提前写 Skill。跑的过程中记一件事：哪一步是靠人反复提醒才没做错的。「验证完记得杀进程」提醒了三次，「改了接口记得同步 Postman 集合」提醒了两次——这些才是候选。没被提醒过的步骤不需要 Skill。

写的时候守住几条：

- 一个 Skill 只覆盖一个工作流。`api-verify` 只做契约核对，不顺手做压测，也不替 `make check` 跑测试。
- 步骤要能被别人照着执行。至少四样东西不能省：前置校验（缺什么就 STOP，写明怎么装）、每步的输入和输出、失败了怎么办（回到哪一步，重跑什么范围）、结束时的清理（进程、临时目录、端口，无论成败都执行）。
- 要提问就先定死提问方式。每次一项、允许跳过、不给默认值这三条一起生效才有用：给了默认值，用户就会一路回车，Skill 收集到的全是模板占位。
- 写完用同一场景验证。找出当初那个漏项发生的场景，加载 Skill 再跑一遍，看漏项是否真的不再发生。没有变化就不要留——一个不改变行为的 Skill 只是多一份要维护的文档。

支持文件按同样的原则处理：需要几十行 shell 才能说清的动作（校验参数、复制、批量替换、重建符号链接）写成脚本放进 Skill 目录，`SKILL.md` 只留调用命令和结果解读。判断标准是这段内容会不会被模型「理解」出偏差——会的就交给脚本，脚本的退出码不会因为上下文变长而改变。

三个信号提示该新建一个 Skill：

- 同一类返工出现第二次。第一次是意外，第二次说明流程本身缺一步。
- 口头约定传不到下一个人手里。「上线前记得改那个配置」只活在一次对话里。
- 新人上手总卡在同一处。卡点固定，说明缺的是步骤而不是经验。

反过来也有三个信号该合并或删掉：

- 两个 Skill 的 Step 开始互相引用，要同时读才能做完一件事，合并成一个。
- 某个 Skill 的步骤已经被命令或 Hook 接管（格式检查进了 `PostToolUse` 之后就不需要一份「记得跑 gofmt」的流程），删掉。
- 描述里的触发条件最近几个版本一次都没成立，删掉。

Skill 数量不是资产。`.ai/ai-rules.md` 的「Skills」表一屏能看完，是个合适的规模。

引入外部 Skill 时先建一份来源记录，写清楚从哪来、固定在哪个版本、许可证是什么、本地有没有改动，本工程用 `SOURCE.md` 放在 Skill 目录里（`go-development` 记录的是 netresearch，版本 1.15.1，本地改动为无）。同时在准则里写死一句：外部 Skill 是参考资料，与 `.ai/ai-rules.md` 冲突时以本文件为准。少了这句，外部文档里的测试规范和依赖建议会悄悄替换掉本工程的门禁定义。
