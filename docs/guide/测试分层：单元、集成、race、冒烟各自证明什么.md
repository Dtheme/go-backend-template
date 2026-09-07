---
title: 测试分层：单元、集成、race、冒烟各自证明什么
type: explainer
series: go-backend-template Spec 开发范式
tags:
  - testing
  - build-tags
  - race
---

# 测试分层：单元、集成、race、冒烟各自证明什么

`go-backend-template` 的 `1.0.0` 在本机 Go 1.27.0 darwin/arm64 上跑过 `make check`、`make test-integration`、`make test-race`、`make smoke`，四条命令全绿。这不是同一件事跑了四遍。单元测试管业务规则和错误映射对不对，集成测试管请求能不能穿过真实 TCP 与中间件链路，race 管共享状态在并发下安不安全，冒烟管真实二进制能不能被装配起来并启动。少跑一层，剩下的绿色只覆盖自己那一层。

## 三层的边界写在构建标签里

`.ai/ai-rules.md` 的「代码规范 / 测试」一节把测试分为三级，隔离手段是 Go 构建标签：

| 层级 | 标签 | 文件名约定 | 命令 | 约束 |
|---|---|---|---|---|
| 单元 | 无 | `xxx_test.go` | `make test` | 不依赖网络、磁盘外部状态与真实服务 |
| 集成 | `//go:build integration` | `xxx_integration_test.go` | `make test-integration` | 使用真实依赖，连接信息来自环境变量，缺失时 `t.Skip` 并说明原因 |
| 端到端（按需） | `//go:build e2e` | 按需 | 无固定目标 | 覆盖跨服务完整流程 |

对应的 `Makefile` 目标就是三条 `go test`：

```makefile
# Makefile
test:
	go test ./...

test-integration:
	go test -tags integration ./...

test-race:
	go test -race ./...
```

`make check` 依赖 `vet test spec-check`，另加一条 `gofmt -l` 检查（见「Command 落地」）。它跑单元层，不跑集成层：命令里没有 `-tags integration`，带标签的文件压根不会被编译。跑什么交给编译器决定，Agent 不用记住哪些测试需要真实依赖。忘记加标签的测试则会漏进 `make check`，要么因依赖缺失失败，要么按上表「缺失时 `t.Skip` 并说明原因」的约定被跳过，只在 `go test -v` 输出里显示为 SKIP。两种情况都说明它不属于单元层，但只有前者会让 `make check` 变红。

`1.0.0` 里只有一个文件带标签：`internal/httpapi/integration_test.go`，首行是 `//go:build integration`。它没有 `xxx_` 前缀，隔离照样成立，`go test` 只认首行标签，文件名后缀是给读者看的约定。仓库里没有 `e2e` 文件，这一层空着，按需再加。

## 单元层：三个包，三条边界

单元测试按依赖方向 `httpapi -> note <- memstore` 落在三个包里，每个包只管自己那条边界。

`internal/note/service_test.go` 有 9 个测试函数。存储被一个 `fakeRepo` 顶掉，时间和 ID 生成走 `svc.now`、`svc.newID` 两个注入接缝：

```go
// internal/note/service_test.go
svc := NewService(repo)
boom := errors.New("entropy exhausted")
svc.newID = func() (string, error) { return "", boom }
```

`TestService_Create_Validation` 的 5 个子用例覆盖 title 缺失、空白、101 个 ASCII、101 个汉字，以及 content 2001 个汉字，断言 `ValidationError` 的 `Field` 与 `Message` 逐字匹配，拒绝之后 `repo` 里没有残留。`TestService_Create_LengthCountsRunes` 走反方向，证明 100 个汉字和 2000 个汉字是合法边界，把需求里「按 Unicode 字符计数，不按字节」钉住。`TestService_Get` 的 4 个子测试分开 `ErrNotFound`、大小写不匹配和存储错误，最后一种必须包装原错误，且不能被误判成 `ErrNotFound`。

`internal/memstore/store_test.go` 有 4 个测试函数：`TestStore_SaveFind_RoundTrip`、`TestStore_Find_Missing`（未知 ID 与大小写不匹配）、`TestStore_ConcurrentSave`（100 个 goroutine）和 `TestStore_Save_DuplicateID`。最后一个来自审查 finding R1：第一版 `Store.Save` 碰到已有 ID 会静默覆盖，与需求「不得覆盖」冲突。修复顺序是先补这个失败测试，再让 `Save` 返回 `note.ErrAlreadyExists`，测试同时断言原始笔记未被改写。

`internal/httpapi/notes_test.go` 有 8 个测试函数，全部用 `httptest.NewRequest` 与 `httptest.NewRecorder` 直接驱动 handler，不开端口。除了注入 `failingRepo` 的 `TestNotes_RepositoryFailureReturns500` 自己调 `NewHandler`，其余都走 `newTestHandler()`。`TestCreateNote_Created` 检查 201、`Content-Type`、四个字段齐全、`id` 匹配 `^n_[0-9a-f]{12}$`、`created_at` 是带 `Z` 后缀的 RFC 3339。`TestCreateNote_BadBody` 有 7 个子用例，其中 `trailing json value` 与 `trailing garbage` 来自 finding R2：`decodeJSON` 原本不管首个 JSON 值之后的尾随数据，修复后用第二次 `Decode` 核对 `io.EOF`。`TestNotes_RepositoryFailureReturns500` 注入一个返回 `disk on fire` 的 `failingRepo`，断言响应是 500 `internal_error`，`error.message` 不含这段内部细节。

三个包各有一个 100 并发的用例：`TestService_Create_ConcurrentUniqueIDs`、`TestStore_ConcurrentSave`、`TestCreateNote_ConcurrentUniqueIDs`。`make test` 下它们照跑，能证明的只有 100 次创建都成功、ID 互不相同、没有丢写。

## 集成层：往返用例只有走真实 TCP 才成立

`internal/httpapi/integration_test.go` 只有 2 个测试函数，`httptest.NewServer(newTestHandler())` 起一个真实监听，`srv.Client()` 发请求。文件头部的注释写明了它的定位：通过真实 TCP 监听与 `http.Client` 走完整请求链路，包括中间件、编码和状态码。

`TestIntegration_Notes` 是一条往返。`POST /v1/notes` 拿到 201 并解析出 `id`，`GET /v1/notes/{id}` 拿到 200 且响应体与创建结果逐字段相等，`n_000000000000` 断言 404 `not_found` 与固定 message `note not found`，空白 title 断言 400 `invalid_argument`。每个响应都过一遍 `doJSON` helper，检查 `Content-Type: application/json; charset=utf-8`。`TestIntegration_HealthzAndPing` 保持既有的健康检查与 ping。

这层证明 `NewHandler` 里 `withRecover(logger, withRequestLog(logger, mux))` 这条链路在真实连接上仍然产生同样的状态码和信封。`cmd/api` 的装配和配置读取不归它管，那是冒烟的事。`1.0.0` 没有外部依赖，集成层就只覆盖真实网络链路。规则要求接入数据库、缓存或外部服务之后，这层必须连真实依赖，覆盖成功与失败路径，不得因依赖未就绪跳过。它与上表「缺失时 `t.Skip` 并说明原因」不打架，两句话管的是不同层次：「代码规范 / 测试」允许集成测试在连接信息缺失时 `t.Skip` 并说明原因，Step 6.1 第 2 项要求自验前先把依赖准备好（连接串环境变量、`/readyz`），自验全部通过才算完成，依赖未就绪时该做的是准备依赖，不是接受 Skip。

## helpers_test.go 是两层的编译并集

`internal/httpapi/helpers_test.go` 不带构建标签，里面只有两个函数：

```go
// internal/httpapi/helpers_test.go
// 无构建标签：单元层与集成层（-tags integration）共用。集成测试专用的 helper 放到带标签的文件里。
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestHandler() http.Handler {
	return NewHandler(testLogger(), note.NewService(memstore.New()))
}
```

编译并集是单向的。`go test -tags integration ./...` 同时编译无标签文件和带标签文件，`integration_test.go` 可以直接复用 `notes_test.go` 里的 `noteIDPattern`、`decodeNote` 和 `assertErrorEnvelope`，`notes_test.go` 顶部的注释也把这些标为「单元层与集成层共用的响应形状与断言」。反过来不成立。`make test` 不带标签，单元测试引用集成文件里的 `doJSON` 会直接编译失败。集成专用 helper 只能留在带标签的文件里。

还有一个容易被忽略的后果：`make test-integration` 会把全部单元测试再跑一遍，无标签文件在任何标签组合下都要编译。单独看 `make test-integration` 的输出，绿色里既有集成用例也有单元用例。

## race 只在有共享状态时才是必需项，count 默认为 1

`make test-race` 是全仓 `go test -race ./...`。规则把它列为并发相关改动的必需门禁，Step 6.1 第 3 项写明「本版本涉及并发改动时执行；无并发改动可说明后跳过」。`1.0.0` 引入了 `memstore.Store` 的 `sync.RWMutex` 加 `map[string]note.Note`，必跑。

三个 100 并发用例在 `-race` 下才回答另一个问题：`Save` 的写锁与 `Find` 的读锁有没有盖住所有共享访问。没有 `-race`，一次丢锁的 map 写入可能恰好没崩；有 `-race`，竞争检测器在第一次并发访问就报出来。需求 F-1.0.0-001 的 AC-5 把这条写进了验收标准：100 个并发创建全部成功且 ID 各不相同，并明确括注 `make test-race` 通过。

反方向也有约束。deterministic 测试默认 `count=1`，只有并发或随机逻辑才用 `-race` 或 `-count` 重复。`TestService_Create_Validation` 这类纯函数用例跑 20 遍不多出任何信息，只会拉长每一次 `make check`。第三方 Skill `go-development` 的 `references/testing.md` 有一节 Race Detection，列了在 `RLock` 下写字段这类常见错误，Step 5 编码时可按需加载；它是参考资料，与 `ai-rules.md` 冲突时以后者为准。

## 冒烟回答的是装配问题

`scripts/smoke.sh` 先 `go build -o bin/api-smoke ./cmd/api`，再用 `HTTP_ADDR=:18080` 启动真实进程，轮询 `/healthz` 最多 50 次，然后执行 9 次 `check`，覆盖 healthz、ping、405、404 与笔记创建、读取、404、400、DELETE 405。逐项的期望状态码与响应子串见「Command 落地」。

它证明的是 `cmd/api/main.go` 里 `note.NewService(memstore.New())` 与 `httpapi.NewHandler(logger, notes)` 的装配、`internal/config` 读 `HTTP_ADDR`，以及进程能在真实端口上应答。断言只是子串匹配，不核对字段类型和完整信封，代替不了单元与集成层。逐接口的契约核对归 Skill `api-verify`，`1.0.0` 的 `api-verify` 在端口 18090 上跑了 17 项场景。退出时 `trap` 杀进程并删掉二进制，不留副作用。

## 测试计划表把 AC 绑到用例名，并在 Step 7 回写

`Specs/technical/技术方案模版.md` 的「测试计划」是一张四列表：层级、文件、用例（场景）、对应验收标准。Step 4 填场景，Step 7 第 6 项要求回写成实际落地的文件与用例名，给下一版本当回归基线。spec coding 里「每条验收项对应 Case」在本模板的落点就是这里。

`Specs/technical/1.0.0/技术方案.md` 回写后的表按文件行标注验收标准组，`internal/httpapi/notes_test.go` 一行对应「F-1.0.0-001 AC-1 至 AC-5；F-1.0.0-002 AC-1 至 AC-3」，`internal/note/service_test.go` 一行对应「F-1.0.0-001 AC-1、AC-2、AC-3、AC-5；F-1.0.0-002 AC-2」。下面这张逐条验收标准到用例名的表，是按各文件内的用例场景整理出来的，技术方案里没有它：

| 验收标准 | 固化它的用例 |
|---|---|
| F-1.0.0-001 AC-1 | `TestCreateNote_Created`、`TestService_Create_ValidInput` |
| F-1.0.0-001 AC-2、AC-3 | `TestCreateNote_ValidationErrors`、`TestService_Create_Validation` |
| F-1.0.0-001 AC-4 | `TestCreateNote_BadBody` |
| F-1.0.0-001 AC-5 | `TestCreateNote_ConcurrentUniqueIDs`、`TestService_Create_ConcurrentUniqueIDs`、`TestStore_ConcurrentSave`，加 `make test-race` |
| F-1.0.0-002 AC-1 | `TestGetNote_RoundTrip`、`TestIntegration_Notes` |
| F-1.0.0-002 AC-2 | `TestGetNote_NotFound`、`TestService_Get`、`TestIntegration_Notes` |
| F-1.0.0-002 AC-3 | `TestNotes_MethodNotAllowed` |

这张表不受 `spec-check` 保护。`spec-check` 的十条规则核对必需文件与章节、符号链接、版本目录与 Feature ID 一致性、「交付状态」块和 Postman 集合结构，都不解析用例名。`1.0.0` 的两处偏差是审查角色 `spec-reviewer` 发现的：第一轮 R4 指出测试计划没有回写用例名，第二轮 R6 指出一个用例名挂错，都由 Controller 修正。两轮审查由通用 Agent 加载 `.claude/agents/spec-reviewer.md` 正文按该角色执行，不是 Claude Code 原生 agents 目录加载。审查结论不替代命令，命令也没覆盖这张表，两者各守一段。

谁跑哪一层，由实现阶段的分工决定。`.claude/agents/spec-implementer.md` 规定实现者只运行 focused 测试 `go test ./internal/<pkg>/...`，返回每个用例的 `red_observed` 与 `green_command`，不运行 `make check`、`make smoke` 和 `api-verify`；这些 broad 门禁由主会话作为唯一执行者运行并记录。`1.0.0` 的实现返回了 11 个改动文件和逐用例的 RED/GREEN 记录，四类门禁在 `graph.json` 里以 record 形式绑定到候选 `2d03ec4f1c56`（见「Spec + Graph 落地」）。实现同样由通用 Agent 加载 `spec-implementer.md` 正文按该角色执行，不是 Claude Code 原生 agents 目录加载。两个 agent 文件的 `tools` 限制尚无端到端记录，`docs/spec-graph/落地.md` 的 1.0.0 实录也注明了这一点。

## 四层全绿之后仍然停在 verifying

| 层 | 命令 | 证明 | 不证明 |
|---|---|---|---|
| 单元 | `make test`（含于 `make check`） | 业务规则、校验、错误映射、handler 的状态码与信封 | 真实连接、装配、并发安全 |
| 集成 | `make test-integration` | 真实 TCP 上的中间件链路与往返 | 真实二进制与配置 |
| race | `make test-race` | 共享状态在并发访问下无数据竞争 | 逻辑正确性之外的任何事 |
| 冒烟 | `make smoke` | 装配、启动、端口应答 | 字段类型与完整契约 |

四条命令全部通过，`api-verify` 的 17 项场景也通过，`go run ./cmd/spec-graph check 1.0.0` 退出码 0。`event 1.0.0 verified` 却被守卫拒绝，输出「守卫不满足: verified: 技术方案「交付状态」user_acceptance 为 pending，需要 confirmed」，退出码 3。graph stage 停在 `verifying`、revision 46，技术方案交付状态是 `stage: verifying`、`user_acceptance: pending`、`review: pass`。测试证明代码符合技术方案，替不了用户确认技术方案符合他的意图。绿色不等于交付，用户确认为什么不可省略见「验证闭环」。

## 工具也要测试

`make check` 依赖 `cmd/spec-check`，可选 Graph 依赖 `cmd/spec-graph`。门禁工具自己出错时，错误会以「通过」的形式出现，这几个包的测试数量反而最多。

`internal/specdoc` 有 7 个测试函数。`TestParseDeliveryStatus_DeliveredRequiresConfirmation` 固化门禁：`delivered` 配 `pending` 或 `changes_required` 必须报错。`TestParseDeliveryStatus_Structure` 用 11 个子用例枚举缺标题、标题重复、非 yaml 块、无代码块、未闭合、未知键、重复键、缺键、非法 stage、非法 review 和行格式错误。`TestParseDeliveryStatus_LongLineNotTruncated` 塞进一行 5 MiB 文本，解析器不用 `bufio.Scanner` 就是因为它。`TestParseDeliveryStatus_HeadingInsideFenceIgnored` 证明围栏内的 `## 交付状态` 不计数，CRLF 文档也能解析。`TestRealTemplateParses` 读真实的 `技术方案模版.md`。

`internal/speccheck` 有 23 个测试函数，覆盖十条规则的正反例。`TestRun_TemplateRepo` 直接对仓库根 `../..` 运行 `Run`，要求零问题，模板自身永远处在被检查状态。`cmd/spec-check/main_test.go` 的 `TestRun_ExitCodes` 钉死三种出口：无问题输出 `spec-check ok (1 version(s))` 退出 0，删掉 `Makefile` 后输出 `spec-check: 1 problem(s)` 退出 1，根目录不存在退出 2。`spec-init` 实录里看到的两行输出就是它们。

`internal/specgraph` 有 33 个测试函数。并发与锁的用例解释了 `make test-race` 为什么是全仓运行而不是只跑业务包：`TestInit_Concurrent` 让 8 个 goroutine 同时 `Init`，要求恰好 1 次成功，其余返回 `ErrConflict`；`TestRecord_Concurrent` 用 16 个并发 `Record`，要求成功次数等于落盘的 evidence 数和 revision；`TestWrite_ExclusiveDuringCommit` 在一次提交过程中嵌套发起第二次写入，断言被拒绝、不落盘、无临时文件残留，且 `graph.json.lock` 已释放。`cmd/spec-graph/main_test.go` 的 `TestRun_ExitCodes` 把 0、1、2、3 四种退出码逐一钉死，包括 `v0.1.0` 这种非法版本号退出 2 和守卫拒绝退出 3。

这些工具由两个 Agent 测试先行实现，再经过对抗审查。`spec-check` 的审查提出 8 条 finding，7 条在 `speccheck` 内修复，1 条属于 `specdoc`，由主会话修复。工具包和业务包用同一条规则：先有失败测试，再有实现。要信任工具给出的结论，先得有人能证明它在该失败的时候会失败。
