MAKEFLAGS = -B
BIN      := sync-email-boxing
PREFIX   ?= /usr/local
BINDIR   := $(PREFIX)/bin
GO       ?= go
LDFLAGS  := -s -w
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)

.PHONY: all build release test vet fmt run clean install uninstall

all: build

## build: 编译当前平台二进制
build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) .

## release: 交叉编译常见平台到 dist/
release:
	rm -rf dist && mkdir -p dist
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)-linux-amd64 .
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)-linux-arm64 .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)-darwin-arm64 .
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)-darwin-amd64 .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o dist/$(BIN)-windows-amd64.exe .

## test: 运行单元测试（不带 LIVE_CFG，不访问网络）
test:
	$(GO) test ./...

## vet: 静态检查
vet:
	$(GO) vet ./...

## fmt: 格式化代码
fmt:
	$(GO) fmt ./...

## run: 编译并运行
run: build
	./$(BIN)

## install: 安装到 $(BINDIR)
install: build
	install -m 0755 $(BIN) $(BINDIR)/$(BIN)

## uninstall: 卸载
uninstall:
	rm -f $(BINDIR)/$(BIN)

## clean: 清理构建产物
clean:
	rm -rf $(BIN) dist
