---
name: api-verify
description: 启动本地 Go 服务，按技术方案的 API 契约（或 Postman 集合）逐接口用 curl 验证状态码、响应字段与错误信封，输出核对报告并清理进程。用于版本开发 Step 6.1 自验和接口改动后的复验。
---

# /api-verify — 本地接口验证

> 用法：Claude Code `/api-verify [version]`，Codex `$api-verify [version]`。
> 不传 version 时验证 Postman 集合与 `技术讲解.md` 接口清单中的全部接口；传入 version 时只验证该版本技术方案「API 契约」中的接口。

## 定位

本 Skill 对**真实进程**做契约核对，是 Step 6.1 的最后一道自验，不替代单元测试与集成测试：每个接口的成功与错误路径都必须先由 `make check` / `make test-integration` 中的测试固化，本 Skill 只确认真实二进制的行为与契约一致。发现测试未覆盖的场景时，先补测试再修代码。

## 前置校验

1. `go version`、`curl --version` 可用；缺失则 STOP 并提示安装
2. `make check` 与 `make test-integration` 已通过；未通过则先回到测试，不做契约核对
3. Postman MCP（可选）：系统提示中存在 `mcp__postman__*` 工具时，Step 2 结束后可额外用 MCP 运行 `api/postman/` 集合并把断言结果并入报告；没有则跳过，不影响结论
4. 读取验证来源：
   - 有 version：`Specs/technical/{version}/技术方案.md` 的「API 契约」一节
   - 无 version：`api/postman/*.postman_collection.json` + `Specs/technical/技术讲解.md` 的「接口清单」
5. 读取 `Specs/requirements/协议与数据.md` 获取错误信封格式与状态码约定
6. 需要外部依赖（数据库等）时，确认依赖已就绪（连接串环境变量、`/readyz` 返回 200）；未就绪则 STOP 并告知

---

## 执行流程

### Step 1 — 启动服务

```bash
PORT="${API_VERIFY_PORT:-18090}"   # 与 scripts/smoke.sh 的 18080 错开；被占用时换一个未使用端口
RUN_DIR="$(mktemp -d)"             # 每次运行独立目录，多次并行不会互相覆盖或误杀
go build -o "$RUN_DIR/api" ./cmd/api
HTTP_ADDR=":$PORT" "$RUN_DIR/api" > "$RUN_DIR/server.log" 2>&1 &
echo $! > "$RUN_DIR/pid"
for i in $(seq 1 50); do curl -fs "http://127.0.0.1:$PORT/healthz" >/dev/null && break; sleep 0.1; done
```

启动失败时读取 `$RUN_DIR/server.log` 定位原因，修复后重试；不要跳过。

### Step 2 — 逐接口验证

对契约中的每个接口，至少执行：

1. **成功路径**：按契约请求体发送，核对 HTTP 状态码、响应 Content-Type、响应字段（名称、类型、必填字段非空）
2. **每种错误场景**：参数缺失 / 格式错误 / 资源不存在 / 冲突等，核对状态码与 `error.code` 与契约一致，`error.message` 不泄露内部信息
3. **方法不匹配**：对同路径发一个未定义的方法，确认 405
4. **幂等或重复提交**（契约声明时）：重复请求确认行为符合声明

curl 模式：

```bash
curl -s -o "$RUN_DIR/body" -w '%{http_code}' -X POST "http://127.0.0.1:$PORT/v1/xxx" \
  -H 'Content-Type: application/json' -d '{"field":"value"}'
cat "$RUN_DIR/body"
```

每个请求记录：方法、路径、请求体摘要、期望状态码、实际状态码、字段核对结果、结论（通过 / 失败）。

### Step 3 — 核对 Postman 集合

- 契约中的每个接口在集合中都有对应请求；缺失的记为「集合待补」
- 集合中的请求 URL、方法、示例请求体与契约一致；不一致的记为「集合待更新」
- 断言脚本至少检查状态码与关键字段
- 有 Postman MCP 时：用 MCP 对运行中的服务执行集合，记录每个请求的断言结果；断言失败但 Step 2 curl 通过，说明集合过期，记为「集合待更新」而不是接口失败

### Step 3.5 — 核对测试覆盖

对 Step 2 中每个「场景」，在 `*_test.go` / `*_integration_test.go` 中找到对应用例并记录用例名；找不到的记为「缺测试」，必须在 Step 6 补齐后才能给出「通过」结论。

### Step 4 — 停止服务并清理

```bash
kill "$(cat "$RUN_DIR/pid")" 2>/dev/null || true
rm -rf "$RUN_DIR"
```

无论验证是否通过都必须执行清理；用 `lsof -i :$PORT` 确认端口已释放。

### Step 5 — 输出报告

```
## API 验证报告 — {version 或 全量}

服务: http://127.0.0.1:{PORT}   启动日志: 正常 / 异常摘要

| 接口 | 场景 | 期望 | 实际 | 对应测试 | 结论 |
| --- | --- | --- | --- | --- | --- |
| POST /v1/users | 成功创建 | 201, id/email | 201, id/email | TestCreateUser_OK | 通过 |
| POST /v1/users | 邮箱缺失 | 400 invalid_argument | 400 invalid_argument | TestCreateUser_MissingEmail | 通过 |
| POST /v1/users | 重复邮箱 | 409 conflict | 500 internal_error | 缺测试 | 失败 |

Postman 集合: 全部一致 / 待补 N 项 / 待更新 N 项（列出）；MCP 运行结果: 未使用 / N 通过 M 失败

结论: 通过 / 失败（列出失败项与缺测试项）
```

### Step 6 — 失败处理

- 任一项失败或缺测试：回到代码，先补失败测试再修，`make check`（触及外部依赖时加 `make test-integration`）通过后只重跑失败接口与其相关场景
- 失败原因是契约本身不合理：不得私自改契约，向用户说明并按「需求变更与新增规则」更新技术方案后再验证
- 报告中「集合待补 / 待更新」项在 Step 7 维护文档时落盘

---

## 与用户验证的关系

本 Skill 是 Agent 自验（Step 6.1）。自验通过后仍需进入用户验证（Step 6.2）：提示用户 `make run` 启动服务，用 Postman（MCP 代跑或手动导入 `api/postman/` 集合）或报告中的 curl 命令自行验证，等待用户最终确认。Postman 是可选工具，用户确认才是必需步骤。
