---
title: API 契约设计：接口在编码前怎样被冻结
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - api-contract
  - http
  - error-envelope
---

# API 契约设计：接口在编码前怎样被冻结

`POST /v1/notes` 在 1.0.0 里返回 `201`，`title` 空白返回 `400 invalid_argument`，不存在的 ID 返回 `404 not_found`。这三条不是 handler 写完之后补记的。它们在编码前就写进技术方案的「API 契约」一节，经用户确认，代码只负责把契约映射成 Go 类型和状态码。

冻结靠三层东西。`Specs/requirements/协议与数据.md` 规定所有接口共用的通用约定；`.ai/ai-rules.md` 的 Step 3 规定每个版本怎样写契约、什么算破坏性变更；`internal/httpapi` 里的 `writeJSON` / `writeError` 与 `decodeJSON` 把约定收成唯一出口。契约归人工需求侧所有，实现只做映射。

## 协议与数据.md 先规定所有接口的共同约束

`Specs/requirements/协议与数据.md` 按 ai-rules 的文件权限规则属于「AI 禁止修改」，文件头部也写明由人工维护、AI 只读。AI 出技术方案、写接口时必须遵守。它不描述某个具体接口，只回答所有接口共同的问题：

| 约定 | 内容 |
|---|---|
| 协议 | HTTP/1.1，请求与响应均为 `application/json; charset=utf-8` |
| 路由前缀 | `/v1/`；破坏性变更升级前缀版本，不在同一版本内改语义 |
| 路由定义 | `METHOD /path` 形式（Go 1.22+ `ServeMux` 模式），方法不匹配返回 405 |
| 时间字段 | RFC 3339，UTC |
| ID 字段 | 字符串，不暴露自增主键语义 |
| 分页 | `?limit=&cursor=`，响应带 `next_cursor`，为空表示结束 |
| 成功响应 | 直接返回资源对象或列表，不包裹 `data` 层 |
| 失败响应 | 统一错误信封 `{"error": {"code": "...", "message": "..."}}` |
| 健康检查 | `GET /healthz` 返回 `{"status":"ok"}`；引入数据库等依赖后新增 `GET /readyz` |

状态码与 `error.code` 的对应关系也在这里固定，HTTP 层不能自己发明新的 code：

| HTTP | error.code | 场景 |
|---|---|---|
| 400 | `invalid_argument` | 参数缺失、格式错误、JSON 解析失败 |
| 401 | `unauthenticated` | 未登录或凭证无效 |
| 403 | `permission_denied` | 无权限 |
| 404 | `not_found` | 资源不存在 |
| 409 | `conflict` | 唯一约束冲突、状态冲突 |
| 429 | `rate_limited` | 触发限流 |
| 500 | `internal_error` | 未预期错误，不暴露内部细节 |
| 503 | `unavailable` | 依赖不可用、过载保护 |

「鉴权」和「对接方约束」两节在模板里只有 HTML 注释占位，业务工程按实际情况填写。1.0.0 只有创建和读取两个接口，没有列表接口，分页约定还没被任何接口用上。`/readyz` 也没实现，当前只有内存存储，没有需要探测的依赖。这两条是留给后续版本的约定，不是已落地的行为。

## Step 3 用四个动作把契约冻结在编码之前

`.ai/ai-rules.md` 的「AI 工作流（版本开发）」把 API 契约设计放在 Step 3，在理解需求之后、出技术方案之前。七个 Step 的全貌见「版本开发工作流」。有接口新增或变更时必须执行；纯内部重构或无接口变更可以跳过，但要说明。四个动作是：

1. 按 `协议与数据.md` 的约定，为每个接口写出方法、路径、鉴权、请求体、成功响应、错误表（HTTP 状态码 + `error.code` + 触发条件）。
2. 检查与已有接口的兼容性：字段删除、语义变化、状态码变化都属于破坏性变更，需升级路径版本前缀或经用户确认。
3. 契约写入技术方案的「API 契约」一节，同时准备 Postman 集合的对应请求，Step 7 落盘。
4. 契约随技术方案一起交用户确认；确认后不得在编码中悄悄改变字段或状态码。

第 2 条给出破坏性变更的定义。`协议与数据.md` 里「不在同一版本内改语义」是同一条规则的另一个落点：一个规定什么算破坏，一个规定破坏了怎么办。第 4 条是冻结本身。确认之后发现契约有问题，走 `api-verify` Skill 写明的那条路：不得私自改契约，向用户说明，按「需求变更与新增规则」更新技术方案后再验证。那条规则要求重新读取需求，更新技术方案里包括「API 契约」在内的相关章节，向用户展示变更并确认，然后才能编码。

「API 契约」也是 `spec-check` 第 7 条规则要求技术方案恰好出现一次的八个二级标题之一。它只保证章节存在，不保证内容与代码一致。一致性由 Step 6.1 的 `api-verify` 对真实进程核对。

## 1.0.0 的两个接口在技术方案里被写成什么

`Specs/technical/1.0.0/技术方案.md` 的「API 契约」一节开头写着「与 `Specs/requirements/协议与数据.md` 一致；此处是实现层面的最终定义」。两个接口的契约合并如下：

| 接口 | 成功 | 错误 |
|---|---|---|
| `POST /v1/notes` | `201`，返回 `id`、`title`、`content`、`created_at` | `400 invalid_argument`：JSON 非法、未知字段、超过 16 KiB（message `invalid request body`）；`title` 缺失 / 空白 / 超过 100 字符（message `title must be 1-100 characters`）；`content` 超过 2000 字符（message `content must be at most 2000 characters`） |
| `GET /v1/notes/{id}` | `200`，同创建响应 | `404 not_found`：ID 不存在（message `note not found`）；`405`：未定义方法，如 `DELETE`（`ServeMux` 文本响应） |

两处细节能看出契约在迁就通用约定，不是迁就实现习惯。请求体超过 16 KiB 映射为 `400` 而不是 `413`，「风险与回滚」一节写明原因：`413` 未在 `协议与数据.md` 的错误码表中。`405` 一行如实标注为 `ServeMux` 文本响应，没有假装它已经带上错误信封。

确认动作也有记录。「需求摘要」写着待确认问题为无，方案确认方式是用户在会话中授权本版本作为模板演示。「变更记录」的第一条记下这次授权，把「交付状态」的 `stage` 改成 `implementing`。

## 契约怎样落到 handler、解码和错误映射

路由注册在 `internal/httpapi/handler.go`，用的就是 `协议与数据.md` 规定的 `METHOD /path` 形式，路径参数由 `r.PathValue("id")` 读取：

```go
// internal/httpapi/handler.go
mux.HandleFunc("GET /healthz", handleHealthz)
mux.HandleFunc("GET /v1/ping", handlePing)

notes := &notesHandler{logger: logger, svc: svc}
mux.HandleFunc("POST /v1/notes", notes.create)
mux.HandleFunc("GET /v1/notes/{id}", notes.get)
```

请求解析集中在 `internal/httpapi/notes.go` 的 `decodeJSON`，它做三件事：`http.MaxBytesReader` 把请求体限制在 `maxBodyBytes = 16 << 10`；`DisallowUnknownFields` 拒绝契约之外的字段；第二次 `Decode` 必须得到 `io.EOF`，否则拒绝首个 JSON 值之后的尾随数据。`decodeJSON` 返回错误时，`create` 一律写 `400 invalid_argument`、message `invalid request body`，与契约表第一行逐字对应。

领域错误到状态码的映射只有一处，`notesHandler.writeServiceError`：

```go
// internal/httpapi/notes.go
case errors.As(err, &ve):
	writeError(w, http.StatusBadRequest, "invalid_argument", ve.Message)
case errors.Is(err, note.ErrNotFound):
	writeError(w, http.StatusNotFound, "not_found", "note not found")
default:
	h.logger.Error("note request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
```

`ve.Message` 来自 `internal/note/note.go`。`ValidationError{Field, Message}` 由 `Service.Create` 构造，常量 `maxTitleRunes = 100`、`maxContentRunes = 2000` 决定 message 里的数字，长度按 `utf8.RuneCountInString` 计数，与需求「按 Unicode 字符计数」一致。`ErrNotFound` 由 `Service.Get` 在 `repo.Find` 返回未找到时返回。`note` 包不 import `net/http`，状态码只出现在 `httpapi`。ai-rules「错误处理」一节就是这么要求的：领域层定义业务错误，HTTP 层集中映射到状态码与 `error.code`。

信封本身在 `internal/httpapi/response.go`，整个文件只有两个函数：

```go
// internal/httpapi/response.go
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: errorBody{Code: code, Message: message}})
}
```

`Content-Type` 的值逐字等于 `协议与数据.md` 的约定。`errorResponse{Error: errorBody{Code, Message}}` 的 JSON tag 就是 `error.code` 与 `error.message`。ai-rules「DRY 红线」的公共实现清单把这两个函数列为 JSON 响应与错误响应的唯一入口，「HTTP 层」一节要求响应只经它们输出，不手写 `w.Write`。

契约冻结不等于实现一次写对。第一轮 `spec-reviewer` 审查的 R2 指出 `decodeJSON` 不检查尾随数据，上面第二次 `Decode` 的检查就是这条 finding 的修复。修复走测试先行，执行者是 `spec-implementer` 角色：通用 Agent 加载 `.claude/agents/spec-implementer.md` 正文来扮演，不是 Claude Code 原生 agents 目录加载。`TestCreateNote_BadBody` 新增 `trailing json value` 与 `trailing garbage` 两个子用例。R1 指出 `memstore.Store.Save` 会静默覆盖同 ID，修复后 `Save` 返回 `note.ErrAlreadyExists`。ID 由服务端生成，客户端触发不到这条路径，它落在 `writeServiceError` 的 `default` 分支，返回 `500` 并记日志，「风险与回滚」一节写明了这点，契约表不用加 `409` 行。审查结论不替代命令，修复后仍由主会话作为唯一 Controller 重跑门禁（见「Subagent 与 Hook 落地」）。

## Postman 集合必须跟着契约走

`api/postman/go-backend-template.postman_collection.json` 在文件权限规则里属于「AI 维护，接口变更时同步」。Step 3 第 3 条要求设计契约时一并准备请求；Step 7 第 3 条要求落盘，并写明「可选验收工具，但集合必须与契约一致」；「日常代码修改」第 4 条同样要求接口变化同步 Postman 集合。

集合的 `info.name` 为 `go-backend-template`。集合变量两个，`baseUrl` 默认 `http://localhost:8080`，另一个是 `noteId`。7 个请求：`GET /healthz`、`GET /v1/ping`、`POST /v1/ping (405)`、`POST /v1/notes 创建笔记`、`GET /v1/notes/{id} 读取笔记`、`GET /v1/notes/{id} 不存在 (404)`、`POST /v1/notes title 为空 (400)`。创建请求的测试脚本断言 `201`、`id` 匹配 `^n_[0-9a-f]{12}$`、`title` 与 `content` 的值、`created_at` 为字符串，再用 `pm.collectionVariables.set('noteId', b.id)` 把 ID 交给读取请求。404 请求断言 `error.code` 为 `not_found`；400 请求断言 `error.code` 为 `invalid_argument`，`error.message` 含 `title`。

创建、404、400 三个请求的断言与技术方案契约表对得上。读取请求只断言 `200` 与 `id` 一致，没核对契约表「同创建响应」要求的 `title`、`content`、`created_at`。契约表 `405` 一行是 `DELETE /v1/notes/{id}`，集合里没有对应请求，唯一的 `405` 请求是 `POST /v1/ping`。

机器只守住集合的结构。`spec-check` 第 9 条规则检查 `api/postman/*.postman_collection.json` 是 JSON 对象、`info.name` 非空字符串、`item` 为数组，不比较请求与契约。内容一致性归 `api-verify` Skill 的 Step 3：契约中每个接口在集合里都要有请求，缺失记为「集合待补」，URL、方法、示例体不一致记为「集合待更新」；有 Postman MCP 时用 MCP 跑集合，断言失败但 curl 通过说明集合过期，记为「集合待更新」而不是接口失败。

Postman 只是用户验证的可选工具，ai-rules 写着本工程的必需门禁不依赖任何 MCP Server（见「MCP 落地」）。1.0.0 的自验没用到 Postman MCP，当时也没有该 MCP 可用。契约核对由 `api-verify` 对真实进程完成，17 项场景全部通过，场景清单与清理结果见「验证闭环」。

## 已知问题：404 与 405 还是 ServeMux 的文本响应

`协议与数据.md` 要求失败响应使用统一错误信封。未匹配路径的 404 和方法不匹配的 405 由 `http.ServeMux` 直接返回纯文本，没经过 `writeError`。`Specs/technical/技术讲解.md` 的「已知问题与待优化项」第一条如实记着这件事，也给了方向：需要统一时在 `NewHandler` 中包一层 fallback handler。技术方案契约表里 `405` 一行标注 `ServeMux 文本响应`，说的是同一件事。

现有验证都只断言状态码，没断这两种响应的 body。`internal/httpapi/handler_test.go` 的 `TestUnknownRouteReturns404`、`TestPingWrongMethodReturns405`，`notes_test.go` 的 `TestNotes_MethodNotAllowed` 覆盖 `DELETE` 与 `PUT`；`scripts/smoke.sh` 里 `GET /not-found` 一条 `404` 检查、`POST /v1/ping` 与 `DELETE /v1/notes/{id}` 两条 `405` 检查，期望 body 子串都为空；Postman 的 `POST /v1/ping (405)` 也只断言 `405`。需求 F-1.0.0-002 的 AC-3 只要求 `DELETE /v1/notes/{id}` 返回 `405`。这些证据能证明状态码正确，证明不了信封已经统一，问题就留在已知问题里，没写成已完成。

按 Step 3 第 2 条的定义，补齐 fallback handler 不删字段、不改状态码。404 / 405 的 body 从 `ServeMux` 纯文本变成错误信封算不算第二项「语义变化」，规则没细到这一层，应在技术方案里说明并经用户确认，不能只凭排除另外两项就推出不属于破坏性变更。改动步骤与「日常代码修改」流程一致：先补断言 body 的失败测试，再改 `NewHandler`，跑 `make check` 与 `make smoke`，接口行为变化后重跑 `api-verify` 对应接口，同步技术讲解与 Postman 断言（见「代码规范与日常修改」）。在那之前契约表继续标 `ServeMux 文本响应`，比提前写成已统一更有用。

`api-verify` 通过、`spec-reviewer` 第二轮 pass、四类门禁在候选 `2d03ec4f1c56` 上通过之后，1.0.0 的 graph 停在 `verifying`，技术方案「交付状态」是 `stage: verifying`、`user_acceptance: pending`、`review: pass`。接口按契约工作已经有证据，版本是否交付还要等用户确认，两件事分开记录（见「验证闭环」）。
