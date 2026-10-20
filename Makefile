# Nerve Wiki 开发命令入口。运行 `make` 或 `make help` 查看所有命令。
# 需兼容 macOS 自带的 GNU Make 3.81。
# 命令按工具链分区：*-go 只需要 Go，*-web 需要 Node（先执行 pnpm install）；
# 不带后缀的 gen、gen-check、lint、fmt 依次执行两个分区。推送前在本地跑 check 与 gen-check，两者合起来是
# 持续集成 server、web 任务的门禁；gen-check 要求生成物已提交，所以不并进 check，在提交之后运行。
# 端到端测试 e2e 另跑（需要 Docker 与 Chromium），对应持续集成的 e2e 任务。

SHELL := /bin/bash
.DEFAULT_GOAL := help

DEV_COMPOSE := docker compose -f deploy/compose.dev.yaml
# 写进 bin/nervewiki 的版本号。默认值与 server/internal/platform/buildinfo 中的相同；发布时指定，例如 make build VERSION=0.1.0
VERSION ?= 0.1.0-dev
GO_LDFLAGS := -X github.com/open-nerve/NerveWiki/server/internal/platform/buildinfo.version=$(VERSION)
GOLANGCI_LINT_VERSION := 2.14.0
BIN_DIR := $(CURDIR)/bin
GOLANGCI_LINT := $(BIN_DIR)/golangci-lint

# 代码生成（docs/v0.1/M0-foundation/04-P4-api-contract.md 3.3）。生成器都在 server/tools 模块里
OAPI_CODEGEN := go tool -modfile=tools/go.mod oapi-codegen
# sqlc 一律不用 cgo 运行：它改用编译成 wasm 的 libpg_query，不需要 C 编译器
SQLC := CGO_ENABLED=0 go tool -modfile=tools/go.mod sqlc
# go -C 切到 server/tools，所以参数都用绝对路径
BODYSHAPEGEN := go -C server/tools run ./bodyshapegen
# 每个模块一个描述文件 api/modules/<模块>.yaml，生成到该模块的 adapter/http/gen
# events 不生成代码：它唯一的操作（事件流）一直开着答复，处理器手写（M5/P2 文档 3.9）
API_MODULES := $(filter-out events,$(basename $(notdir $(wildcard api/modules/*.yaml))))
# 在读 Makefile 时展开：每个模块的 http/gen 目录里先有 oapi-codegen.yaml，所以新模块的目录也在其中。
# postgres/gen 没有这样的文件：模块第一次有 sqlc 查询时，这个目录要到 gen-go 运行后才出现，不在检查之列；
# 那时没提交的生成代码会让持续集成的编译失败，由编译兜底。git 的 pathspec 不展开 *，这里必须是展开后的路径
GEN_GO_OUT := server/internal/platform/httpserver/apigen $(wildcard server/internal/modules/*/adapter/http/gen) \
	$(wildcard server/internal/modules/*/adapter/postgres/gen)
GEN_WEB_OUT := api/dist web/packages/api-client/src/schema.gen.ts
# 生成物必须已提交且没有差异（未跟踪的新文件也算）；$(1) 是生成物的路径
check-committed = test -z "$$(git status --porcelain -- $(1))" || { git status --short -- $(1); git --no-pager diff -- $(1); echo "生成物与接口描述不一致：执行 make gen，并提交生成的文件"; exit 1; }

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

.PHONY: run
run: ## 以 dev 配置启动服务（先执行 make dev-db）；Ctrl-C 优雅停止
	cd server && NWIKI_ENV=dev go run ./cmd/nervewiki serve

.PHONY: web-dev
web-dev: ## 前端开发服务器 127.0.0.1:5173（热更新），接口代理到 make run 的后端
	pnpm --filter @nervewiki/web dev

# 两个进程都在前台，Ctrl-C 同时发给它们
.PHONY: dev
dev: dev-db ## 一条命令起开发环境：开发数据库、后端（make run）、前端热更新（make web-dev）
	$(MAKE) -j2 run web-dev

.PHONY: build-web
build-web: ## 构建前端，产物在 web/apps/web/dist（需要 Node）
	pnpm --filter @nervewiki/web build

# webui/dist 里只提交 .gitkeep：先清掉上一次复制进去的前端，再复制这一次的。
# 构建参数与 deploy/Dockerfile 的相同（静态链接、-trimpath），改一处要同步另一处：e2e 测的就是要发布的那种构建
.PHONY: build
build: build-web ## 构建 bin/nervewiki，前端内嵌在其中，版本号取 VERSION（需要 Go 与 Node）
	find server/internal/platform/webui/dist -mindepth 1 ! -name .gitkeep -delete
	cp -R web/apps/web/dist/. server/internal/platform/webui/dist/
	cd server && CGO_ENABLED=0 go build -trimpath -ldflags "$(GO_LDFLAGS)" -o ../bin/nervewiki ./cmd/nervewiki

# 故事读 NWIKI_E2E_VERSION，核对 make build 注入的版本号（docs/v0.1/M0-foundation/06-P6-e2e-delivery.md 3.4）
.PHONY: e2e
e2e: build ## 构建 bin/nervewiki，运行端到端故事（需要 Docker 与 Playwright 的 Chromium，见 README）
	cd e2e && NWIKI_E2E_VERSION=$(VERSION) pnpm exec playwright test

# 镜像的标签跟着 VERSION；构建上下文是仓库根目录，见 deploy/Dockerfile 与 .dockerignore。
# 提交信息取自带进构建上下文的 .git：git worktree 的 .git 是指向别处的文件，在那里构建会失败，所以先检查
IMAGE ?= nervewiki:$(VERSION)

.PHONY: image
image: ## 构建镜像 $(IMAGE)，版本号取 VERSION（只需要 Docker）
	@test -d .git || { echo "make image 要在普通的克隆中运行：这里的 .git 不是目录（git worktree），镜像构建取不到提交信息"; exit 1; }
	docker build -f deploy/Dockerfile --build-arg VERSION=$(VERSION) -t $(IMAGE) .

.PHONY: image-smoke
image-smoke: image ## 在镜像上跑 S1、S3：迁移、探针、前端、实例与提交信息、注册关闭、管理员建账户、非 root、优雅停机、附件目录的卷与不可写时拒绝启动（需要 Docker、curl、jq、openssl）
	deploy/image-smoke.sh $(IMAGE) $(VERSION)

.PHONY: tools
tools: ## 安装锁定版本的 golangci-lint 到 ./bin
	@set -o pipefail; \
	if $(GOLANGCI_LINT) --version 2>/dev/null | grep -q "version $(GOLANGCI_LINT_VERSION) "; then \
		echo "golangci-lint $(GOLANGCI_LINT_VERSION) 已安装"; \
	else \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/v$(GOLANGCI_LINT_VERSION)/install.sh | sh -s -- -b $(BIN_DIR) v$(GOLANGCI_LINT_VERSION); \
	fi

.PHONY: gen
gen: gen-go gen-web ## 重新生成全部代码：Go 接口层、api/dist、TS 客户端

.PHONY: gen-go
gen-go: ## 由 api/ 生成 Go 接口层和请求体结构表，由 server/sqlc.yaml 生成查询代码（只需要 Go）
	rm -f server/internal/platform/httpserver/apigen/*.gen.go server/internal/modules/*/adapter/http/gen/*.gen.go
	rm -rf server/internal/modules/*/adapter/postgres/gen
	cd server && $(SQLC) generate
	cd server && $(OAPI_CODEGEN) -config internal/platform/httpserver/apigen/oapi-codegen.yaml ../api/common.yaml
	@set -e; for m in $(API_MODULES); do \
		gen=$(CURDIR)/server/internal/modules/$$m/adapter/http/gen; \
		echo "cd server && $(OAPI_CODEGEN) -config internal/modules/$$m/adapter/http/gen/oapi-codegen.yaml ../api/modules/$$m.yaml"; \
		(cd server && $(OAPI_CODEGEN) -config internal/modules/$$m/adapter/http/gen/oapi-codegen.yaml ../api/modules/$$m.yaml); \
		echo "$(BODYSHAPEGEN) -config $$gen/oapi-codegen.yaml -out $$gen/bodyshape.gen.go $(CURDIR)/api/modules/$$m.yaml"; \
		$(BODYSHAPEGEN) -config $$gen/oapi-codegen.yaml -out $$gen/bodyshape.gen.go $(CURDIR)/api/modules/$$m.yaml; \
	done

.PHONY: gen-web
gen-web: ## 打包 api/dist/openapi.yaml，生成 TS 客户端的类型（需要 Node）
	REDOCLY_SUPPRESS_UPDATE_NOTICE=true pnpm run bundle:api
	pnpm --filter @nervewiki/api-client gen

.PHONY: gen-check
gen-check: gen-check-go gen-check-web ## 重新生成全部代码，检查生成物已提交且没有差异

.PHONY: gen-check-go
gen-check-go: gen-go ## 重新生成 Go 接口层并检查（持续集成 server 任务）
	@$(call check-committed,$(GEN_GO_OUT))

.PHONY: gen-check-web
gen-check-web: gen-web ## 重新生成 api/dist 和 TS 类型并检查（持续集成 web 任务）
	@$(call check-committed,$(GEN_WEB_OUT))

.PHONY: check
check: lint knip test test-go-short build-web ## 静态检查、未使用代码检查、测试、go test -short 的单元测试（CI 的 short 一份）、前端构建（生成物一致性另跑 gen-check）

.PHONY: lint
lint: lint-go lint-web ## 全部静态检查

.PHONY: fmt
fmt: tools ## 修正全部格式：Go（gofmt、goimports）与其余文件（oxfmt，需要 Node）
	cd server && $(GOLANGCI_LINT) fmt ./...
	pnpm run fix:format

# config verify 按 schema 校验 .golangci.yml：run 会静默忽略拼错的键。go mod tidy -diff 在 go.mod 或 go.sum 不整洁时失败。
# server/tools 是嵌套的独立模块，server 的 ./... 不进入它，另跑一次
.PHONY: lint-go
lint-go: tools ## 校验 golangci-lint 配置并运行（含格式检查，server 与 server/tools）；检查 go.mod 整洁
	cd server && $(GOLANGCI_LINT) config verify && $(GOLANGCI_LINT) run ./...
	cd server/tools && $(GOLANGCI_LINT) run --config ../.golangci.yml ./...
	cd server && go mod tidy -diff
	cd server/tools && go mod tidy -diff

.PHONY: lint-web
lint-web: ## Markdown 样例集自检；oxlint（零警告）；格式检查；各包的类型检查（需要 Node）
	node tools/md-fixtures/check.mjs
	pnpm run check:lint
	pnpm run check:format
	pnpm -r run check:types

# 门禁：有未使用的文件、导出、依赖，或配置本身过时（例如不再需要的忽略项），都会失败
.PHONY: knip
knip: ## 检查未使用的文件、导出和依赖（需要 Node）
	pnpm exec knip --treat-config-hints-as-errors

# -race 需要 cgo：macOS 需要 Xcode 命令行工具，Linux 需要 gcc。
# 集成测试用 testcontainers 启动 PostgreSQL，需要 Docker；只跑单元测试用 make test-go-short（cd server && go test -short ./...，
# README 的写法；make check 与 CI 的 short 一份都跑它）
# 不用测试缓存：它不跟踪 server/ 之外的文件，契约测试读取的 api/dist/openapi.yaml 改了也会重放旧结果。
# server/tools 是嵌套的独立模块，另跑一次
.PHONY: test
test: test-go test-web ## 全部测试

# 竞态检测下的测试：一个包至多 15 分钟，短于 CI 作业的时限，卡住时打出正在跑的测试：组合根的整个程序测试在竞态检测下要几分钟，默认的 10 分钟曾经不够（M7/P3B）
GO_TEST_RACE = go test -race -count=1 -timeout 15m

.PHONY: test-go
test-go: ## 运行 Go 测试，含集成测试与 server/tools（开启竞态检测，不用测试缓存；需要 Docker）
	@# 本机与 CI 同样分份（M9/P2 设计 6.5）：组合根之外的包一个 go test，组合根按测试名三份、各一个 go test 二进制、各受 15 分钟，
	@# 同一条规则管两处（之前组合根是一个二进制，本机 8 到 9.5 分钟，比 CI 的任何一份先到 15 分钟）。竞态检测的这四个同时跑
	@# （-j4，随份数；macOS 的 make 3.81 也有 -j），之后不带竞态检测的两份依次跑，不与竞态检测下的构建抢 CPU；各份之和就是下面 CI 的六份
	@# （M8/P3A 审查 C 的 I-1）。M9/P2 S1 各量两次：两份时竞态检测的三个同时跑（-j3）整个 759 与 754 秒、依次 1031 与 1061 秒，
	@# 只快 4.8 分钟，不到设计定的 5 分钟，所以先是依次；分成三份之后（S1 修复第一轮）四个同时跑整个 698 与 730 秒、依次 1024 与 1058 秒，
	@# 同时的平均快 5.45 分钟，过了 5 分钟，所以同时（四个里第一份最长，284 与 319 秒）。同时跑的输出交错：找 --- FAIL、FAIL 与 make 的 Error 三种行
	@$(MAKE) --no-print-directory -j4 test-go-race test-go-bootstrap-1 test-go-bootstrap-2 test-go-bootstrap-3
	@$(MAKE) --no-print-directory test-go-costs
	@$(MAKE) --no-print-directory test-go-guards

# CI 的六份（short 是第七份，只跑单元测试，不在 make test-go 里，见 test-go-short）：一个作业跑全部时超过了 25 分钟的时限（M8/P3A，竞态检测下的组合根占了一半）。
# 组合根之外的包由 go list 列出，以后新加的包自动归入第一份。不带竞态检测的那一份在 M8/P4A 之前作业已到 15 分钟
# （测试一步 13:37），超过 15 分钟就分（M8/P4 设计 3.11）：组合根的预算与守卫各自编译、各起数据库，分作一份。
# 竞态检测下的组合根在 M8/P4A 的 CI 上测试一步 12:00–12:05，离 go test 的 15 分钟只剩约两成，P5 还要加测试（M8/P4A 审查 C 的 M-6）：
# 按测试名分成两份，本机两份的测试时间之和各约一半（M8 收尾时以 TestA 到 TestD 为界，各自单独跑 201 与 206 秒，在 make test-go 里 213 与 217 秒）。
# M9/P1 S1 的整个程序测试几乎都落在第二份（单独跑 213 与 250 秒），界线照 M9/P1 设计 6.1 往后挪到 TestEveryR 之前：
# 名字按字母排在它之前的一份（TestA 到 TestD、TestEa 到 TestEu、TestEveryA 到 TestEveryQ），其余的一份；整个 TestE 挪过去第一份就多出三成
# M9/P1 S3 时各单独跑一次是 213 与 242 秒（第二份多 13%），界线照设计 6.1 再往后挪到 TestEveryS 之前，是重新平衡，
# 不是 S3 的测试让第二份变慢（它们不带竞态检测共约 3 秒）：TestEveryR 开头的两个（vault 的导出与导入，约 18 秒）挪到第一份，
# 挪了之后单独跑 238 与 229 秒。单独跑一次的时间前后可差一成以上（同一棵树上第一份 213 到 260 秒，CI 上同一份 5:44 到 7:59），
# 以后判断挪不挪，取每份两次的平均（M9/P1 S3 审查 C 的 N-4）。
# 一份过了线（CI 两次运行的平均里测试一步过了约 7:30，go test 的 -timeout 15m 的一半）时照 M9/P2 设计 6.5 的规则：
# 只在邻居收下之后两份都在线之下时挪界线，否则按测试名再分一份，从不提高 -timeout。M9/P2 S1 时 CI 的两次运行里第一份的测试一步
# 6:59 与 6:56（平均 6:57.5），第二份 7:15 与 8:33（平均 7:54），过了线；本机各单独跑两次，第一份 256 与 281 秒，第二份 245 与 247 秒。
# S1 先把界线往后挪到 TestO 之前（第二份最前的一段，本机约 16.5 秒，乘 CI 与本机之比 1.73 约 29 秒，估计两份约 7:26 与 7:25）；
# 挪了之后 CI 的三次运行里两份是 7:25 与 7:54、5:25 与 5:27、7:25 与 7:45，第二次两份都快，看作 runner 的离群（S1 修复第一轮，
# 主持者的判断）：第二份照旧过线、第一份就在线上，这次挪动不成立，照规则撤回、按测试名再分一份。第一份回到 TestEveryS 之前，
# 原来的第二份以 BOOTSTRAP_THIRD 分成两份：名字以 TheF 到 TheZ、Ti 到 Tz、U 到 Z 开头的一份，其余的一份（竞态下编译的 374 个测试
# 依次 168、104、102 个，S1 的两次 -json 运行里每个测试的用时之和 265.0、125.1、118.7 秒，新的两份差 5%，在一成之内）；
# 分了之后各单独跑两次，第一份 254 与 246 秒、第二份 121 与 132 秒、第三份 121 与 131 秒（平均 250、126.5 与 126 秒，新的两份差 0.4%）；
# CI 的两次运行里三份的测试一步 5:51 与 7:07（平均 6:29）、5:13 与 4:20（平均 4:46.5）、3:12 与 3:49（平均 3:30.5），都在线之下，
# 这次分照规则成立。第一份离线最近（这个界线上 CI 的四次是 6:59、6:56、5:51、7:07），过了线时照规则先看第二份收不收得下它的一段
# -run 与 -skip 用同一组表达式：第一、第三份 -run 各自的表达式，第二份 -skip 它们的并，三份之和总是整个包，新加的测试
# 自动落在其中一份；一份过了线时照 M9/P2 设计 6.5 的规则改这些表达式
BOOTSTRAP_FIRST = ^Test([A-D]|E([a-u]|very[A-R]))
# M9/P2 S1：第二份的测试一步过了 go test 的 15 分钟的一半（CI 的两次运行 7:15 与 8:33），照规则按测试名再分一份，不提高 -timeout（M9/P2 设计 6.5）
BOOTSTRAP_THIRD = ^Test(The[F-Z]|T[i-z]|[U-Z])
.PHONY: test-go-race
test-go-race: ## 竞态检测下运行组合根（internal/bootstrap）之外的全部 Go 包（CI 的一份；需要 Docker）
	cd server && $(GO_TEST_RACE) $$(go list ./... | grep -v '/internal/bootstrap$$')

.PHONY: test-go-bootstrap
test-go-bootstrap: test-go-bootstrap-1 test-go-bootstrap-2 test-go-bootstrap-3 ## 竞态检测下运行组合根的整个程序测试，三份依次（需要 Docker）

.PHONY: test-go-bootstrap-1
test-go-bootstrap-1: ## 竞态检测下运行组合根的整个程序测试中名字匹配 BOOTSTRAP_FIRST 的（CI 的一份；需要 Docker）
	cd server && $(GO_TEST_RACE) -run '$(BOOTSTRAP_FIRST)' ./internal/bootstrap

.PHONY: test-go-bootstrap-2
test-go-bootstrap-2: ## 竞态检测下运行组合根的整个程序测试中名字既不匹配 BOOTSTRAP_FIRST 也不匹配 BOOTSTRAP_THIRD 的（CI 的一份；需要 Docker）
	cd server && $(GO_TEST_RACE) -skip '$(BOOTSTRAP_FIRST)|$(BOOTSTRAP_THIRD)' ./internal/bootstrap

.PHONY: test-go-bootstrap-3
test-go-bootstrap-3: ## 竞态检测下运行组合根的整个程序测试中名字匹配 BOOTSTRAP_THIRD 的（CI 的一份；需要 Docker）
	cd server && $(GO_TEST_RACE) -run '$(BOOTSTRAP_THIRD)' ./internal/bootstrap

.PHONY: test-go-costs
test-go-costs: ## 不带竞态检测的分配、耗时预算与界线的守卫，以及 server/tools（CI 的一份；需要 Docker）
	@# 竞态检测让同一段代码的分配多出数倍、耗时慢数倍：分配与耗时的预算，以及阅读视图在各种解析下的 HTML 检查与体积上限，在不带它的构建中另测一次
	cd server && go test -count=1 -run '^TestCheckCostsAboutTheBody$$' ./internal/platform/httpserver/bodyshape
	@# /mcp 的请求体的 JSON 值有上限，扫描过了它就停：路由上限那么长（37.8 MB）的 [[],[],…] 被拒时只分配几 KiB（M9/P2 S1 审查 A2-I1）；这个测试只在不带竞态检测的构建里
	cd server && go test -count=1 -run '^TestTheValuesBoundStopsTheScanEarly$$' ./internal/platform/httpserver/mcpserver
	cd server && go test -count=1 -run '^TestTheCostsAreAboutTheSize$$' ./internal/platform/markdown
	cd server && go test -count=1 -run '^(TestALinkWrittenManyTimesResolvesOnce|TestDistinctPathsToPagesOfOneTitleResolveInTimeAsLongAsThey)$$' ./internal/modules/linking/app
	cd server && go test -count=1 -run '^TestTheLinktextsOfManyPagesOfOneTitleAreCheap$$' ./internal/modules/linking/domain
	cd server && go test -count=1 -run '^(TestSubtreeReadsWithoutStatisticsInTimeAsLongAsIt|TestSubtreeOfAFolderOfMostNodesReadsInTimeAsLongAsIt|TestListTrashReadsInTimeAsLongAsAPage|TestTrashLinesReadInTimeAsLongAsAPage|TestRestoreNodesTakesItsTimeAtAnyStatistics|TestTrashHistoryCountsInTimeAtAnyStatistics|TestTheLogReadsInTimeAsLongAsAPage|TestTheRevertsArrayReadsInTimeAtTheUnitsBound)$$' ./internal/modules/page/adapter/postgres
	@# 搜索的守卫单独一行、带 -v：通过时也打出各个读的中位数，CI 的日志里看得到，下次重新测量有数据（M8/P5A 审查 C 的 M-2）
	cd server && go test -count=1 -v -run '^TestTheSearchReadsInTimeAtAnyStatistics$$' ./internal/modules/page/adapter/postgres
	@# 概况的守卫同样带 -v：计数与最后的变更两条语句、读整棵树各自的中位数（M9/P1 设计 3.17.3）
	cd server && go test -count=1 -v -run '^TestTheOverviewReadsInTimeAsLongAsTheTree$$' ./internal/modules/page/adapter/postgres
	cd server && go test -count=1 -run '^TestAnImportsUnitNumbersNamesThatCutAlikeInTimeAsLongAsThey$$' ./internal/modules/page/app
	@# glob 的守卫同样带 -v：最坏的模式在守卫树上走到 MaxGlobWork 的用时（M9/P1 设计 3.10.1）；edit 的匹配的守卫也在这一行：每种形状的用时（M9/P2 设计 3.9）
	cd server && go test -count=1 -v -run '^(TestReplaceStaysWithinItsTime|TestTheWorstGlobsStopWithinTheirWork)$$' ./internal/modules/page/domain
	cd server && go test -count=1 -run '^TestPagesWhoseNamesCutAlikeAreRenamedInTimeAsLongAsThey$$' ./internal/modules/transfer/domain
	go -C server/tools test -race -count=1 ./...

.PHONY: test-go-guards
test-go-guards: ## 不带竞态检测的组合根的预算与界线的守卫（CI 的一份；需要 Docker）
	@# 这一份的 go test 不带 -timeout，每一行受 Go 默认的每个测试二进制 10 分钟。一行在 CI 上的估计（本机两次的平均乘 2.4，这一份的 CI 与本机之比）
	@# 过了它的一半就一个测试一行，单独一行仍过的那一行加 -timeout 15m（M9/P2 设计 6.5）。markdown 的三个原先一行，本机 140 到 155 秒；
	@# M9/P2 S1 各单独跑两次：成本的 125.5 与 127.9 秒，渲染的 3.7 与 4.0 秒，链接的 22.5 与 21.7 秒，之和乘 2.4 约 6.1 分钟，所以拆成三行；
	@# 成本的一个乘 2.4 约 5.1 分钟，仍过了 Go 默认的每个测试二进制 10 分钟的一半，它的一行加 -timeout 15m
	cd server && go test -count=1 -timeout 15m -run '^TestTheAppsMarkdownCostsAboutItsSize$$' ./internal/bootstrap
	cd server && go test -count=1 -run '^TestTheAppsMarkdownRendersCheckedHTML$$' ./internal/bootstrap
	cd server && go test -count=1 -run '^TestTheAppsLinksAreWithinTheirBound$$' ./internal/bootstrap
	@# 恢复在单元上限的测试单独一行、带 -v：通过时也打出恢复用的时间，CI 的日志里看得到（M8/P2A 第二轮复核 M-a）
	cd server && go test -count=1 -v -run '^TestARestoreAtTheUnitsBoundTakesItsTime$$' ./internal/bootstrap
	@# 撤销在单元上限的测试同样单独一行、带 -v：撤销一次删除用的时间（M8/P3 设计 3.11），与撤销在单轮步数上限时恢复那么多项用的时间（M8/P3A 审查 C 的 I-2）
	cd server && go test -count=1 -v -run '^(TestARevertAtTheUnitsBoundTakesItsTime|TestARevertOfManyTopsAtItsBoundTakesItsTime)$$' ./internal/bootstrap
	@# 后台撤销在 MaxJobItems 的守卫各单独一行、带 -v：请求，整个任务（1 万个条目）与最大的一步（一个最上层的删除）用的时间（M8/P4 设计 3.11）
	cd server && go test -count=1 -v -run '^TestStartRevertAtMaxJobItems$$' ./internal/bootstrap
	cd server && go test -count=1 -v -run '^TestARevertJobOfAnImportAtMaxJobItems$$' ./internal/bootstrap
	@# batch 的推后的改写在 MaxBatchRewrites 的守卫单独一行、带 -v：界线上的单元与整个调用用的时间（M9/P3 设计 3.8.2）；
	@# 同一行还有链接的界线 MaxBatchRewriteLinks：过一条被拒、在界线上做完，单元用的时间只打出、不判（M9/P3 审查 B1-M3）
	cd server && go test -count=1 -v -run '^(TestABatchAtItsRewriteBoundTakesItsTime|TestABatchPastItsLinksBoundIsRefused)$$' ./internal/bootstrap

.PHONY: test-go-short
test-go-short: ## 只跑 server 的单元测试：go test -short，不带竞态检测，不起数据库（README 的写法；make check 与 CI 的一份跑它，不在 make test-go 里）
	@# pgtest 在 -short 下跳过要数据库的测试；按耗时判定的守卫也跳过：-short 的各包同时跑、抢 CPU，CI 上一个 2 秒的界线到过 2.6 秒
	@# （S1 修复第二轮），它们由 test-go-costs 与 test-go-guards 各自单独一行跑。别的照常跑。README 与上面的注释答应过贡献者这样跑得通，
	@# 以前没有哪个门禁跑它：M9/P2 S1 的一个测试在 -short 下失败，到 S1 的修复第一轮才被看见（修复第二轮，审查 C 的 C2-M1），
	@# 所以 CI 单独一份跑它。最慢的包本机约 28 秒（组合根与 httpserver），乘 2.4 约 70 秒，远在 Go 默认的每个测试二进制 10 分钟的一半之下，
	@# 不加 -timeout（M9/P2 设计 6.5）。本机整个 53 与 38 秒。make check 也跑它：check 与 gen-check 合起来是 CI 的 server、web 任务的门禁
	@# （文件头与 README），少了这一份就不是（修复第三轮，审查 C 的 C3-M1）
	cd server && go test -short -count=1 ./...

.PHONY: test-web
test-web: ## 运行前端各包的测试（vitest；需要 Node）
	pnpm -r run test
