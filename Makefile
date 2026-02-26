.PHONY: dev dev-down dev-logs prod prod-down install-tools

HUMANLOG := $(HOME)/go/bin/humanlog

# Install dev tooling
install-tools:
	go install github.com/humanlogio/humanlog/cmd/humanlog@latest

# Start full dev cluster with hot-reload and pretty logs
dev:
	docker compose -f docker-compose.dev.yml up --build 2>&1 | $(HUMANLOG)

# Start in background (detached)
dev-bg:
	docker compose -f docker-compose.dev.yml up --build -d

# Tail logs from running dev cluster, pretty-printed
dev-logs:
	docker compose -f docker-compose.dev.yml logs --follow --no-log-prefix 2>&1 | $(HUMANLOG)

# Stop dev cluster
dev-down:
	docker compose -f docker-compose.dev.yml down --remove-orphans

# --- Production ---

prod:
	docker compose up -d

prod-down:
	docker compose down
