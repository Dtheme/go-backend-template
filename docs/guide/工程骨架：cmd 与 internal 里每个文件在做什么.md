---
title: 工程骨架：cmd 与 internal 里每个文件在做什么
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - go
  - architecture
  - net-http
  - slog
---

# 工程骨架：cmd 与 internal 里每个文件在做什么

去掉测试与工具包，`go-backend-template` 的服务代码是 8 个文件、396 行。`cmd/api/main.go` 58 行，`internal/config/config.go` 27 行，`internal/httpapi` 四个文件合计 180 行，`internal/note/note.go` 95 行，`internal/memstore/store.go` 36 行。module 名 `example.com/go-backend-template`，`go.mod` 声明 `go 1.25.0`，本机验证运行时 Go 1.27.0 darwin/arm64。import 里没有第三方包。

Spec 工作流守的就是这 396 行。`.ai/ai-rules.md` 的「包组织与依赖方向」只允许 `cmd -> httpapi -> domain <- storage`，反向 import 算设计错误。`1.0.0` 演示版本新增 `internal/note` 与 `internal/memstore`，`spec-implementer` 角色返回 11 个改动文件；技术方案「文件清单」里生产 Go 代码的落点只有 `note.go`、`store.go`、`notes.go`、`handler.go`、`main.go` 五个。下面逐个说它们做什么、不做什么，哪条规则或哪个测试在守它。

## 八个文件的分工

| 文件 | 行数 | 职责 | 对应测试 |
| --- | --- | --- | --- |
| `cmd/api/main.go` | 58 | logger、配置、装配 `memstore` 与 `note.Service`、`http.Server`、信号与优雅停机 | `make smoke` 启动真实二进制 |
| `internal/config/config.go` | 27 | 从 `HTTP_ADDR`、`SHUTDOWN_TIMEOUT` 两个环境变量装配 `Config` | `config_test.go` 3 个用例 |
| `internal/httpapi/handler.go` | 28 | `NewHandler` 注册 4 条路由并套中间件；`healthz`、`ping` handler | `handler_test.go` 6 个用例 |
| `internal/httpapi/notes.go` | 84 | `notesHandler` 的 `create`、`get`，`decodeJSON`，领域错误到状态码的映射 | `notes_test.go` 8 个测试函数 |
| `internal/httpapi/response.go` | 25 | `writeJSON`、`writeError` 与统一错误信封 | `TestWriteErrorEnvelope` |
| `internal/httpapi/middleware.go` | 43 | `withRequestLog` 请求日志、`withRecover` panic 兜底 | `TestRecoverMiddlewareReturnsJSON500` |
| `internal/note/note.go` | 95 | `Note`、`CreateInput`、`Repository` 端口、`Service`、错误类型、ID 生成 | `service_test.go` 9 个测试函数 |
| `internal/memstore/store.go` | 36 | `note.Repository` 的内存实现，`sync.RWMutex` 加 `map` | `store_test.go` 4 个测试函数 |

`internal/httpapi/integration_test.go` 带 `//go:build integration`，用 `httptest.NewServer` 走真实 TCP 覆盖整条中间件链。`helpers_test.go` 不带标签，`testLogger` 与 `newTestHandler` 供两层共用。四层测试各自证明什么见「测试分层」。

## cmd/api/main.go：只做装配，不写业务

`cmd/api/main.go` 第 20 到 32 行是全部装配：

```go
logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

cfg := config.Load()
notes := note.NewService(memstore.New())

server := &http.Server{
	Addr:              cfg.HTTPAddr,
	Handler:           httpapi.NewHandler(logger, notes),
	ReadHeaderTimeout: 5 * time.Second,
	ReadTimeout:       10 * time.Second,
	WriteTimeout:      10 * time.Second,
	IdleTimeout:       60 * time.Second,
}
```

logger 是进程里唯一的 `*slog.Logger`，JSON 格式写 stdout，之后只通过参数传给 `httpapi.NewHandler`，没有全局变量。规则「日志与配置」要求只用 `log/slog`。`note.NewService(memstore.New())` 这一行是 `1.0.0` 加进来的：`memstore.New()` 返回 `*memstore.Store`，`NewService` 的参数类型是 `note.Repository` 接口，具体存储类型只在这里出现一次。四个超时写死为常量，不进 `Config`。整个仓库里只有 `main` 同时 import `config`、`httpapi`、`memstore`、`note` 四个包。

第 34 行之后是生命周期。`errCh` 容量 1，goroutine 里先记 `server starting`，再把 `ListenAndServe` 的返回值送进去。主 goroutine 用 `signal.Notify` 监听 `SIGINT` 与 `SIGTERM`，然后 `select`：监听失败记 `server failed`，退出码 1；收到信号记 `shutting down` 与信号名。随后用 `cfg.ShutdownTimeout` 建 context 调 `server.Shutdown`，让在途请求跑完；返回错误且不是 `http.ErrServerClosed` 时同样退出码 1，否则记 `server stopped`。容量 1 保证 `Shutdown` 之后 `ListenAndServe` 返回的 `ErrServerClosed` 能放进 channel，goroutine 不会阻塞泄漏。规则「后台 goroutine 必须有退出路径」在入口文件里就是这个数字。

## internal/config：两个环境变量，非法值静默回退

| 字段 | 环境变量 | 默认值 | 采用条件 |
| --- | --- | --- | --- |
| `HTTPAddr` | `HTTP_ADDR` | `:8080` | 非空即采用 |
| `ShutdownTimeout` | `SHUTDOWN_TIMEOUT` | `10s` | `time.ParseDuration` 成功且大于 0 |

`Load()` 返回 `Config`，不返回 error，包内没有 logger。`SHUTDOWN_TIMEOUT` 写成 `not-a-duration` 就回退到 `10s`，不会有任何输出。`TestLoadInvalidTimeoutFallsBack` 把这个行为钉住，`TestLoadFromEnv` 用 `t.Setenv` 验证 `:9090` 与 `5s` 的覆盖。`config` 只 import `os` 与 `time`。规则的 DRY 表把「环境变量配置」指向 `config.Load`，业务包不直接 `os.Getenv`。技术讲解写明新增配置项要同时改 `Config`、`Load`、`config_test.go`、配置表与 README。`make smoke` 与 `api-verify` 分别用 `HTTP_ADDR=:18080` 和端口 18090 起真实进程，走的就是这条读取路径。

## internal/httpapi：路由、解析、信封、中间件各占一个文件

`handler.go` 的 `NewHandler` 是 HTTP 层唯一的导出函数：

```go
func NewHandler(logger *slog.Logger, svc *note.Service) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealthz)
	mux.HandleFunc("GET /v1/ping", handlePing)

	notes := &notesHandler{logger: logger, svc: svc}
	mux.HandleFunc("POST /v1/notes", notes.create)
	mux.HandleFunc("GET /v1/notes/{id}", notes.get)

	return withRecover(logger, withRequestLog(logger, mux))
}
```

第二个参数是具体类型 `*note.Service`，不是接口。测试注入点在更下一层：`notes_test.go` 的 `TestNotes_RepositoryFailureReturns500` 用返回 `disk on fire` 的 `failingRepo` 构造真实 `Service`，500 分支照样验得到，`Service` 就不必再套一层接口，规则里也不允许为「将来可能需要」创建无调用方抽象。路由用 Go 1.22+ 的 `METHOD /path` 模式，未知路径 404 与方法不匹配 405 交给 `ServeMux`。中间件顺序固定：`withRecover` 最外，`withRequestLog` 其次，`mux` 最内。`handleHealthz` 与 `handlePing` 各一行，返回 `{"status":"ok"}` 与 `{"message":"pong"}`。

`notes.go` 定义 `maxBodyBytes = 16 << 10`、`notesHandler{logger, svc}`、请求结构 `createNoteRequest{title, content}` 与响应结构 `noteResponse{id, title, content, created_at}`。JSON 标签只出现在这一层，领域类型 `note.Note` 没有标签。`create` 先 `decodeJSON`，失败一律 400 `invalid_argument`、message 固定为 `invalid request body`，成功后调用 `svc.Create(r.Context(), note.CreateInput{...})` 并以 201 返回。`get` 用 `r.PathValue("id")` 取路径参数交给 `svc.Get`。`decodeJSON` 做三件事：`http.MaxBytesReader` 限 16 KiB，超限表现为 `Decode` 出错、映射成 400 而不是 413；`DisallowUnknownFields` 拒绝未知字段；第一次 `Decode` 之后再 `Decode` 一次并要求得到 `io.EOF`。第三件来自审查 finding R2，`TestCreateNote_BadBody` 的 7 个子用例里 `trailing json value` 与 `trailing garbage` 就是它的 RED/GREEN 记录。领域错误在 `writeServiceError` 集中映射：

| 领域错误 | 判断方式 | HTTP | `error.code` | `error.message` |
| --- | --- | --- | --- | --- |
| `*note.ValidationError` | `errors.As` | 400 | `invalid_argument` | `ve.Message`，如 `title must be 1-100 characters` |
| `note.ErrNotFound` | `errors.Is` | 404 | `not_found` | `note not found` |
| 其他，含 `note.ErrAlreadyExists` 与存储故障 | `default` | 500 | `internal_error` | `internal server error`，细节只进日志 |

`response.go` 只有两个函数。`writeJSON` 设置 `Content-Type: application/json; charset=utf-8`、写状态码、`Encode`。`writeError` 把 `code` 与 `message` 装进 `errorResponse{Error: errorBody{...}}`，输出 `{"error":{"code":...,"message":...}}`，与 `Specs/requirements/协议与数据.md` 的信封一致。`_ = json.NewEncoder(w).Encode(body)` 是规则「错误处理」唯一允许吞掉 error 的地方，规则原文的例外条件是「写响应的 `Encode` 错误除外，已在 `writeJSON` 集中处理」。DRY 表要求 handler 只经 `writeJSON` / `writeError` 输出，不手写 `w.Write`。

`middleware.go` 的 `statusRecorder` 内嵌 `http.ResponseWriter`，`WriteHeader` 时记下状态码，默认 200。`withRequestLog` 在 `next.ServeHTTP` 返回后记一条 `request`，字段只有 `method`、`path`、`status`、`duration_ms`，没有请求体和 header。需求「非功能需求」要求笔记正文不进日志，`api-verify` 的 17 项场景跑完核对过服务器日志，确实不含正文。`withRecover` 用 `defer recover()` 兜底，记 `panic recovered`，再用 `writeError` 返回 500 `internal_error`。规则限定它只兜程序缺陷，业务错误不得用 `panic`。

## internal/note：规则和端口在一个文件里

`note.go` 第 43 到 56 行定义端口与服务：

```go
type Repository interface {
	Save(ctx context.Context, n Note) error
	Find(ctx context.Context, id string) (Note, bool, error)
}

type Service struct {
	repo  Repository
	now   func() time.Time
	newID func() (string, error)
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now, newID: newID}
}
```

`Note{ID, Title, Content, CreatedAt}` 与 `CreateInput{Title, Content}` 是不带标签的普通结构体。`Repository` 由领域包定义、存储包实现。`Find` 用第二个返回值 `bool` 区分「不存在」与「查询失败」，`Service.Get` 据此把 `!ok` 翻译成 `ErrNotFound`，把 error 用 `find note: %w` 包装上抛。`TestService_Get` 的 `repo error` 子用例断言后者不会被误判成 `ErrNotFound`。

`Service` 的 `now` 与 `newID` 是未导出字段，`NewService` 固定填 `time.Now` 与包内 `newID`，只有同包测试能替换。`TestService_Create_ValidInput` 换成固定的东八区时间，验证 `CreatedAt` 落库时已是 UTC；`TestService_Create_IDGenerationFails` 换成返回错误的函数，验证失败时不写 repo。没有 clock 接口，也没有 options 结构。

`Create` 的规则与需求逐条对应。`strings.TrimSpace` 只处理 `title`，`content` 原样保存。长度用 `utf8.RuneCountInString` 计数，`title` 1 到 `maxTitleRunes`（100），`content` 至多 `maxContentRunes`（2000），对应需求「按 Unicode 字符计数，不按字节」。超限返回 `&ValidationError{Field, Message}`，message 用常量拼成 `title must be 1-100 characters` 与 `content must be at most 2000 characters`，需求要求错误信息指明字段名。`ValidationError.Error()` 直接返回 `Message`，HTTP 层原样透出即可。校验通过后调用 `s.newID()`，以 `s.now().UTC()` 组装 `Note`，`repo.Save` 的错误用 `save note: %w` 包装。

`ErrNotFound` 与 `ErrAlreadyExists` 是包级哨兵。`ErrAlreadyExists` 定义在领域包而不是 `memstore`，错误词汇表归领域层所有，存储层只负责返回它。`TestService_Create_SaveAlreadyExists` 断言包装后 `errors.Is` 仍成立，且不是 `ValidationError`。`newID` 从 `crypto/rand` 读 6 字节，`hex` 编码后加前缀 `n_`，恰好 12 个小写十六进制字符，匹配需求的 `^n_[0-9a-f]{12}$`。技术方案「风险与回滚」写明 Go 1.24 起 `rand.Read` 不返回错误，这个 error 返回值只作为测试注入接缝保留，审查 finding R3 修正过这条描述。

## internal/memstore：一把 RWMutex 和一张 map

`store.go` 的 `Store` 只有两个字段：`mu sync.RWMutex` 与 `notes map[string]note.Note`，`New()` 初始化 map。第 15 行 `var _ note.Repository = (*Store)(nil)` 是编译期断言，端口签名一旦变化，`memstore` 在任何测试运行之前就编译失败。`Save` 持写锁，先查 `s.notes[n.ID]` 是否存在，存在返回 `note.ErrAlreadyExists`，否则写入。`Find` 持读锁，返回 `(n, ok, nil)`，内存实现产生不了 error。两个方法都用 `_` 忽略 `context.Context`，锁内没有 I/O，对应规则「锁的范围最小，且不在持锁时做 I/O」。`Save` 的完整片段「Subagent 与 Hook 落地」已经引用。

存在即拒绝的分支来自审查 finding R1。第一版 `Save` 直接赋值，ID 冲突会静默覆盖，与需求「并发创建互不影响，不得丢失或覆盖」冲突。返回错误比覆盖更严格地满足这句话，没有放宽需求，不需要用户决定。修复先补 `TestStore_Save_DuplicateID`，它断言第二次 `Save` 返回 `ErrAlreadyExists` 且第一条笔记原样保留。`TestStore_ConcurrentSave` 用 100 个 goroutine 写入，`make test-race` 在 `-race` 下跑同一批用例。数据只在进程内存里，重启即丢，需求「背景与目标」已经声明这是 `1.0.0` 的范围。

## 依赖方向写在 import 里，不写在文档里

规则里的箭头可以直接用 `go list` 核对：

```text
go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./cmd/api ./internal/config ./internal/httpapi ./internal/note ./internal/memstore
```

| 包 | 引用的项目内包 | 没有引用 |
| --- | --- | --- |
| `cmd/api` | `config`、`httpapi`、`memstore`、`note` | 三个工具包 |
| `internal/httpapi` | `note` | `memstore`、`config` |
| `internal/note` | 无 | `net/http`、任何存储包 |
| `internal/memstore` | `note` | `httpapi`、`net/http` |
| `internal/config` | 无 | 任何项目内包 |

`note` 是唯一有两个依赖方、又没有任何项目内依赖的包，`cmd -> httpapi -> note <- memstore` 的两条箭头都指向它。`httpapi` 的生产代码从不接触 `memstore`，只有 `helpers_test.go` 的 `newTestHandler` 用它装配测试 handler。换成技术讲解里举例的 `internal/postgres/` 时，生产侧只需新包实现 `note.Repository`，再改 `main.go` 里 `memstore.New()` 这一处。Go 的 `internal` 目录规则另外挡住外部 module 的 import。技术方案「模块设计」表列的三个包与关键类型，和这张 import 表是同一份事实的两种写法。`spec-check` 只核对文档，代码边界靠 import 图与测试守住。

## 工具包只共享 go.mod，不共享代码

`internal/specdoc`（`ParseDeliveryStatus`）、`internal/speccheck`（`speccheck.go`、`versions.go`，十条规则）、`cmd/spec-check`（`-root` 参数，退出码 0、1、2）、`internal/specgraph`（`graph.go`、`digest.go`、`store.go`、`transition.go`、`check.go`）与 `cmd/spec-graph`（`init` / `status` / `record` / `finding` / `event` / `check`，退出码 0、1、2、3）和服务代码放在同一个 module 里。对这五个包做同样的 `go list` 核对：`cmd/spec-check` 只引用 `speccheck`，`cmd/spec-graph` 只引用 `specgraph`，这两个包再各自引用 `specdoc`；五个包都不引用服务代码，`cmd/api` 也不引用它们。同一 module 的意义在门禁。`make check` 的 vet 与单测覆盖它们，`spec-graph` 的 candidate 摘要覆盖 `cmd` 与 `internal` 全部普通文件，改工具代码同样会让候选身份变化。`spec-check` 十条规则见「Command 落地」，`spec-graph` 的阶段、守卫与摘要规则见「Spec + Graph 落地」。

## 已知问题

- 404 与 405 由 `ServeMux` 返回纯文本，不走 JSON 信封；`TestUnknownRouteReturns404`、`TestPingWrongMethodReturns405` 与 `scripts/smoke.sh` 都只断言状态码。技术讲解记为待优化，建议在 `NewHandler` 里包一层 fallback handler。
- 没有 `/readyz`；`协议与数据.md` 约定引入数据库等依赖后再补。
- `withRecover` 在 `withRequestLog` 外层，panic 会绕过后者末尾的 `logger.Info`，panic 的请求只有 `panic recovered` 一行，没有 `request` 行。
- `ErrAlreadyExists` 没有单独映射成 409，走 `default` 分支返回 500；技术方案「风险与回滚」写明 48 位随机 ID 的冲突概率可忽略，这是有意的选择。
- `config.Load` 对非法 `SHUTDOWN_TIMEOUT` 静默回退，没有日志。
- `docs/spec-graph/理论.md` 与 `docs/spec-graph/落地.md` 已存在但尚未被 git 跟踪，`git status` 为 `??`；技术讲解与 README 对 `docs/spec-graph/落地.md` 的引用、`.ai/ai-rules.md` 对 `docs/spec-graph/` 的引用在工作区内都有落点，提交前 clone 副本里仍会缺失。
- `1.0.0` 停在 graph stage `verifying`、revision 46，技术方案交付状态为 `user_acceptance: pending`，上面的代码是已通过全部门禁但尚未交付的候选。

新增一个领域的顺序与 `1.0.0` 相同：先建 `internal/{domain}/` 放类型、规则与 `Repository`，再建 `internal/{storage}/` 实现它，然后在 `NewHandler` 注册路由，最后在 `main.go` 装配。改完重跑上面那条 `go list`，箭头方向不变才算落位。
