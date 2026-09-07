---
name: init-project
description: 从 go-backend-template 模板新建业务工程。完成目录复制、Go module 重命名、项目信息填充、验证与 git 初始化；支持在已初始化的工程中再次运行补充信息。
---

# /init-project — 业务工程初始化

> 一个命令完成从模板到业务工程的初始化：文件级重命名 + 项目信息填充 + 验证。
> 用法：Claude Code `/init-project`，Codex `$init-project`。

## 交互规则

- **每次只问一个配置项**，等用户回复后再问下一个
- 直接输出文字提问，等待用户在对话框中输入
- 用户输入 `跳过` 时保留占位，继续下一项；首次运行的必填项不接受跳过
- 不提供默认值或建议值

---

## 执行流程

### Step 0 — 检测当前状态

读取当前工作目录的 `go.mod`：

- module 为 `example.com/go-backend-template` → 当前目录是模板工程，执行 Step 1（复制 + 重命名），`PROJECT_ROOT` 在 Step 1 结束后确定
- 其他 module → 当前目录已是业务工程，跳过 Step 1，`PROJECT_ROOT` = 当前目录绝对路径
- 没有 `go.mod` → 不是 Go 工程，提示用户并停止

**扫描占位符**（基于 `PROJECT_ROOT`）：

| 文件 | 检测模式 |
| --- | --- |
| `.ai/ai-rules.md` | 「项目简介」仍含「模板工程」 |
| `Specs/technical/技术讲解.md` | 「项目概述」的服务名称仍为 `go-backend-template`，描述仍为模板描述 |
| `README.md` | 标题仍为 `go-backend-template` 或含「模板工程」 |
| `api/postman/*.postman_collection.json` | `info.name` 仍为 `go-backend-template` |
| `CLAUDE.md` / `AGENTS.md` | 符号链接不存在或断裂 |

需要执行 Step 1，或 `.ai/ai-rules.md` 仍含「模板工程」，视为**首次运行**；否则为**后续运行**，只展示尚未配置的项。

> Step 1 执行后，后续所有文件操作必须基于 `PROJECT_ROOT` 的绝对路径，不是模板目录。

---

### Step 1 — 复制与 module 重命名

仅当检测到当前目录为模板工程时执行。

**逐一询问**：

1. 新工程目录名（英文，字母、数字、`-`、`_`，不能以数字开头）
2. Go module 路径（例如 `github.com/org/service-name`；不能是 `example.com/go-backend-template`）

**执行**：

```bash
TEMPLATE_ROOT="$(pwd)"
bash "$TEMPLATE_ROOT/.ai/skills/init-project/rename_module.sh" "$NEW_DIR_NAME" "$NEW_MODULE" "$(dirname "$TEMPLATE_ROOT")"
```

脚本会：复制模板到同级目录、删除 `.git` 与 `bin/`、替换 module 路径与模板名、重建符号链接、运行 `go build` 与 `go test` 验证。

完成后：

- 输出 `PROJECT_ROOT = {DEST_PARENT}/{NEW_DIR_NAME}`
- 输出脚本替换的文件清单
- 记住 `PROJECT_ROOT`，继续 Step 2

---

### Step 2 — 收集项目信息

标注「必填」的项首次运行不可跳过。

| 询问项 | 说明 | 更新目标 |
| --- | --- | --- |
| （必填） 服务名称 | 业务工程的英文名 | `ai-rules.md` 项目简介、`技术讲解.md` 项目概述、`README.md` 标题、Postman `info.name` |
| （必填） 一句话描述 | 服务做什么、给谁用 | `ai-rules.md` 项目简介、`技术讲解.md` 项目概述、`README.md` |
| 默认监听端口 | 覆盖 `HTTP_ADDR` 默认值 | `internal/config/config.go`、`config_test.go`、`README.md`、`技术讲解.md` 配置表、Postman `baseUrl` |
| 主要外部依赖 | 计划接入的数据库 / 消息队列 / 第三方服务（仅记录，不安装） | `技术讲解.md` 项目概述 |
| 鉴权方式 | 例如 Bearer token / 无 | 提示用户填写 `Specs/requirements/协议与数据.md` 的「鉴权」一节（AI 不改该文件） |

首次运行先收集必填项，再逐一询问可选项；后续运行只展示仍为占位的项。

---

### Step 3 — 更新文件内容

只更新用户提供了值的项，使用 Edit 精确替换，不改动无关内容。所有路径基于 `PROJECT_ROOT`。

| 文件 | 更新内容 |
| --- | --- |
| `.ai/ai-rules.md` | 「项目简介」第一段改为业务描述；保留语言、依赖、架构、验证四行 |
| `Specs/technical/技术讲解.md` | 「项目概述」服务名称、描述；如提供外部依赖则追加一行「计划接入」 |
| `README.md` | 标题与首段改为业务工程描述；删除「从模板新建业务工程」一段及其 `rename_module.sh` 命令块（副本已不是模板） |
| `api/postman/*.postman_collection.json` | `info.name`、`info.description`、必要时 `baseUrl` |
| `internal/config/config.go` + `config_test.go` | 用户提供端口时同步默认值与测试断言 |

修改 Go 文件后运行 `make check` 确认通过。

---

### Step 4 — 收尾

1. **git 初始化**（仅首次运行，且 `PROJECT_ROOT` 内无 `.git`）：

   ```bash
   cd "$PROJECT_ROOT" && git init && git add . && git commit -m "chore: initialize project from go-backend-template"
   ```

2. **确认双 Agent 结构**：
   - `.ai/ai-rules.md` 存在
   - `CLAUDE.md` 与 `AGENTS.md` 是指向 `.ai/ai-rules.md` 的符号链接，断裂则重建
   - `.claude/skills` 与 `.agents/skills` 指向 `../.ai/skills`，断裂则重建
   - `.claude/settings.json`（含 `permissions.allow` 与 `PostToolUse` Hook）、`.claude/agents/spec-reviewer.md`、`.claude/agents/spec-implementer.md`、`.codex/config.toml` 存在
   - 在 `PROJECT_ROOT` 运行 `make check`：`spec-check` 应输出 `spec-check ok (0 version(s))`

3. **输出报告**：
   - 新工程路径
   - 已配置项及其值
   - 仍为占位的项（提示后续运行 `/init-project` 补充）
   - 修改的文件清单
   - 下一步：在 `Specs/requirements/` 复制需求模版写第一版需求，然后要求 AI 开始版本开发

4. **提示**：如仍有未配置项，说明「部分可选配置尚未设置，后续可运行 `/init-project` 补充」；否则说明「所有配置项已设置完毕」。
