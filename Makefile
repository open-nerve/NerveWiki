# Nerve Wiki 开发命令入口。运行 `make` 或 `make help` 查看所有命令。
# 需兼容 macOS 自带的 GNU Make 3.81。
# 命令按工具链分区：*-go 只需要 Go，*-web 需要 Node（先执行 pnpm install）；
# 不带后缀的 lint 依次执行两个分区，供本地使用。

SHELL := /bin/bash
.DEFAULT_GOAL := help

DEV_COMPOSE := docker compose -f deploy/compose.dev.yaml
GOLANGCI_LINT_VERSION := 2.14.0
BIN_DIR := $(CURDIR)/bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint

.PHONY: help
help: ## 列出所有命令
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: dev-db
dev-db: ## 启动开发数据库（PostgreSQL 18），等待就绪
	$(DEV_COMPOSE) up -d --wait db

.PHONY: dev-db-down
dev-db-down: ## 停止开发数据库，保留数据
	$(DEV_COMPOSE) down

.PHONY: dev-db-reset
dev-db-reset: ## 停止开发数据库并删除数据卷
	$(DEV_COMPOSE) down -v

.PHONY: tools
tools: ## 安装锁定版本的 golangci-lint 到 ./bin
	@set -o pipefail; \
	if $(GOLANGCI_LINT) --version 2>/dev/null | grep -q "version $(GOLANGCI_LINT_VERSION) "; then \
		echo "golangci-lint $(GOLANGCI_LINT_VERSION) 已安装"; \
	else \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/v$(GOLANGCI_LINT_VERSION)/install.sh | sh -s -- -b $(BIN_DIR) v$(GOLANGCI_LINT_VERSION); \
	fi

.PHONY: lint
lint: lint-go lint-web ## 运行全部静态检查

.PHONY: lint-go
lint-go: tools ## 运行 golangci-lint（含格式检查）
	cd server && $(GOLANGCI_LINT) run ./...

.PHONY: lint-web
lint-web: ## Markdown 样例集自检；tools/ 下脚本的 oxlint（零警告）；格式检查（需要 Node）
	node tools/md-fixtures/check.mjs
	pnpm run check:lint
	pnpm run check:format

# 门禁：有未使用的文件、导出、依赖，或配置本身过时（例如不再需要的忽略项），都会失败
.PHONY: knip
knip: ## 检查未使用的文件、导出和依赖（需要 Node）
	pnpm exec knip --treat-config-hints-as-errors

.PHONY: test
test: ## 运行 Go 测试（不用测试缓存）
	cd server && go test -count=1 ./...
