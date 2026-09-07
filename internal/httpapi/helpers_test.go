package httpapi

import (
	"io"
	"log/slog"
	"net/http"

	"example.com/go-backend-template/internal/memstore"
	"example.com/go-backend-template/internal/note"
)

// 无构建标签：单元层与集成层（-tags integration）共用。集成测试专用的 helper 放到带标签的文件里。
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestHandler() http.Handler {
	return NewHandler(testLogger(), note.NewService(memstore.New()))
}
