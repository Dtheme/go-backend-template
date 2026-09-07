.PHONY: run build test test-integration test-race check fmt vet lint vuln smoke spec-check spec-init clean

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api

test:
	go test ./...

test-integration:
	go test -tags integration ./...

test-race:
	go test -race ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

check: vet test spec-check
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:" && gofmt -l . && exit 1)

spec-check:
	go run ./cmd/spec-check

spec-init:
	@test -n "$(VERSION)" || (echo "用法: make spec-init VERSION=x.y.z" && exit 2)
	sh scripts/spec-init.sh $(VERSION)

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run --timeout 5m || echo "golangci-lint 未安装，跳过（可选）"

vuln:
	@command -v govulncheck >/dev/null 2>&1 && govulncheck ./... || echo "govulncheck 未安装，跳过（可选）"

smoke:
	bash scripts/smoke.sh

clean:
	rm -rf bin
