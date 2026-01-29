.PHONY: dev build run generate clean monty-ollama monty-gateway monty-tunnel monty-rag monty-services monty-status restart

# Monty backend host configuration
MONTY_HOST ?= monty
MONTY_GATEWAY_PORT ?= 8001
MONTY_FRONTEND_PORT ?= 3000
MONTY_RAG_DIR ?= /home/n0ko/montyWorldWide

# Check/start Ollama on monty
monty-ollama:
	@echo "Checking Ollama on $(MONTY_HOST)..."
	@ssh $(MONTY_HOST) "pgrep -x ollama > /dev/null || (ollama serve > /tmp/ollama.log 2>&1 &)" && \
		echo "Ollama running on $(MONTY_HOST)"

# Check/start PostgreSQL and RAG database on monty
monty-rag:
	@echo "Checking RAG database on $(MONTY_HOST)..."
	@ssh $(MONTY_HOST) "systemctl is-active postgresql > /dev/null || sudo systemctl start postgresql"
	@ssh $(MONTY_HOST) "sudo -u postgres psql -d monty -c 'SELECT 1' > /dev/null 2>&1 || \
		(sudo -u postgres createdb monty 2>/dev/null; \
		 sudo -u postgres psql -d monty -c 'CREATE EXTENSION IF NOT EXISTS vector;' 2>/dev/null; \
		 sudo -u postgres psql -d monty -c 'CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\";' 2>/dev/null; \
		 cat $(MONTY_RAG_DIR)/rag/migrations/001_initial_schema.sql | sudo -u postgres psql -d monty; \
		 cat $(MONTY_RAG_DIR)/rag/migrations/002_indexes.sql | sudo -u postgres psql -d monty; \
		 cat $(MONTY_RAG_DIR)/rag/migrations/003_functions.sql | sudo -u postgres psql -d monty)"
	@echo "RAG database ready on $(MONTY_HOST)"

# Check/start Monty gateway on monty
monty-gateway: monty-rag
	@echo "Checking Monty gateway on $(MONTY_HOST):$(MONTY_GATEWAY_PORT)..."
	@ssh $(MONTY_HOST) "ss -tlnp | grep -q ':$(MONTY_GATEWAY_PORT) ' || \
		(cd $(MONTY_RAG_DIR) && HTTP_PORT=$(MONTY_GATEWAY_PORT) nohup .venv/bin/python -m uvicorn gateway.main:app --host 0.0.0.0 --port $(MONTY_GATEWAY_PORT) > /tmp/monty-gateway.log 2>&1 &)" && \
		sleep 2 && echo "Monty gateway running on $(MONTY_HOST):$(MONTY_GATEWAY_PORT)"

# Set up SSH tunnel for frontend access via monty
monty-tunnel:
	@echo "Setting up SSH tunnel $(MONTY_HOST):$(MONTY_FRONTEND_PORT) -> localhost:$(MONTY_FRONTEND_PORT)..."
	@ssh $(MONTY_HOST) "ss -tln | grep -q ':$(MONTY_FRONTEND_PORT) '" || \
		ssh -f -N -R 0.0.0.0:$(MONTY_FRONTEND_PORT):localhost:$(MONTY_FRONTEND_PORT) $(MONTY_HOST)
	@echo "Tunnel active: $(MONTY_HOST):$(MONTY_FRONTEND_PORT)"

# Check status of all monty services
monty-status:
	@echo "=== Monty Services Status ==="
	@echo -n "Ollama: " && ssh $(MONTY_HOST) "pgrep -x ollama > /dev/null && echo 'running' || echo 'stopped'"
	@echo -n "PostgreSQL: " && ssh $(MONTY_HOST) "systemctl is-active postgresql"
	@echo -n "RAG DB: " && ssh $(MONTY_HOST) "sudo -u postgres psql -d monty -c 'SELECT 1' > /dev/null 2>&1 && echo 'ready' || echo 'not ready'"
	@echo -n "Gateway: " && curl -s $(MONTY_HOST):$(MONTY_GATEWAY_PORT)/health 2>/dev/null || echo "not running"
	@echo -n "Tunnel: " && pgrep -f "ssh -f -N -R.*:$(MONTY_FRONTEND_PORT)" > /dev/null && echo "active" || echo "inactive"

# Start all monty backend services
monty-services: monty-ollama monty-gateway monty-tunnel
	@echo "All Monty services ready"

# Generate templ files and run server (with monty backend)
dev: monty-services
	templ generate && go run .

# Generate templ files
generate:
	templ generate

# Build binary
build:
	templ generate && go build -o bin/server .

# Run built binary
run:
	./bin/server

# Restart server (kill existing, regenerate, and run with monty services)
restart: monty-services
	@echo "Stopping existing server..."
	-@pkill -x frontend 2>/dev/null
	-@fuser -k 3000/tcp 2>/dev/null
	@sleep 1
	@echo "Regenerating and restarting..."
	templ generate && go run .

# Watch for changes (requires templ watch)
# Set SERVER_HOST and SERVER_PORT env vars to customize
watch:
	templ generate --watch --proxy="http://$${SERVER_HOST:-0.0.0.0}:$${SERVER_PORT:-3000}" --cmd="go run ."

# Clean build artifacts
clean:
	rm -rf bin/
