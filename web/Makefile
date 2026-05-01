# ── 变量 ──
APP_NAME := domain-platform
BUILD_DIR := ./bin
GO := go

# ── 构建 ──
.PHONY: build
build:
	@echo "🔨 building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build -ldflags="-s -w" -o $(BUILD_DIR)/$(APP_NAME) ./cmd/server
	@echo "✅ built: $(BUILD_DIR)/$(APP_NAME)"

.PHONY: run
run: build
	@echo "🚀 starting..."
	$(BUILD_DIR)/$(APP_NAME)

.PHONY: dev
dev:
	@echo "🔧 dev mode..."
	$(GO) run ./cmd/server

# ── 依赖服务 ──
.PHONY: deps-up
deps-up:
	docker compose up -d clickhouse redis

.PHONY: deps-down
deps-down:
	docker compose down

.PHONY: deps-logs
deps-logs:
	docker compose logs -f clickhouse redis

# ── 全部启动 ──
.PHONY: all-up
all-up:
	docker compose up -d
	@sleep 2
	$(MAKE) run

# ── 清理 ──
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)
	rm -rf data/

# ── 测试 ──
.PHONY: test
test:
	$(GO) test ./... -v

# ── 初始化 ──
.PHONY: init
init:
	@mkdir -p data
	@cp -n .env.example .env 2>/dev/null || true
	@echo "✅ initialized, run 'make deps-up' then 'make run'"
