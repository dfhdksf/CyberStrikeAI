BINARY    := cyberstrike-ai
CONFIG    := config.yaml
PID_FILE  := .cyberstrike.pid
LOG_FILE  := cyberstrike.log
VENV_DIR  := venv

ifdef HTTP
  HTTPS_FLAG :=
else
  HTTPS_FLAG := --https
endif

.PHONY: build start stop restart status logs clean setup dev

build:
	@echo ">>> Building $(BINARY)..."
	@go build -o $(BINARY) cmd/server/main.go
	@echo ">>> Build complete"

setup:
	@if [ ! -d "$(VENV_DIR)" ]; then \
		echo ">>> Creating Python venv..."; \
		python3 -m venv $(VENV_DIR); \
	fi
	@echo ">>> Installing Python dependencies..."
	@. $(VENV_DIR)/bin/activate && pip install -r requirements.txt -q
	@echo ">>> Setup complete"

start: build
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo ">>> Already running (PID: $$(cat $(PID_FILE)))"; \
		exit 1; \
	fi
	@echo ">>> Starting $(BINARY) in background..."
	@nohup ./$(BINARY) -config $(CONFIG) $(HTTPS_FLAG) > $(LOG_FILE) 2>&1 & echo $$! > $(PID_FILE)
	@sleep 1
	@if kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo ">>> Started (PID: $$(cat $(PID_FILE)))"; \
		echo ">>> Log: $(LOG_FILE)"; \
	else \
		echo ">>> Failed to start, check $(LOG_FILE)"; \
		rm -f $(PID_FILE); \
		exit 1; \
	fi

stop:
	@if [ ! -f $(PID_FILE) ]; then \
		PID=$$(pgrep -f "./$(BINARY)" 2>/dev/null); \
		if [ -n "$$PID" ]; then \
			echo ">>> Stopping (PID: $$PID)..."; \
			kill $$PID; \
			echo ">>> Stopped"; \
		else \
			echo ">>> Not running"; \
		fi; \
		exit 0; \
	fi
	@PID=$$(cat $(PID_FILE)); \
	if kill -0 $$PID 2>/dev/null; then \
		echo ">>> Stopping (PID: $$PID)..."; \
		kill $$PID; \
		sleep 2; \
		if kill -0 $$PID 2>/dev/null; then \
			echo ">>> Force killing..."; \
			kill -9 $$PID; \
		fi; \
		echo ">>> Stopped"; \
	else \
		echo ">>> Process not running (stale PID file)"; \
	fi; \
	rm -f $(PID_FILE)

restart: stop
	@sleep 1
	@$(MAKE) start

status:
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo ">>> Running (PID: $$(cat $(PID_FILE)))"; \
	else \
		echo ">>> Not running"; \
		rm -f $(PID_FILE) 2>/dev/null; \
	fi

logs:
	@if [ -f $(LOG_FILE) ]; then \
		tail -f $(LOG_FILE); \
	else \
		echo ">>> No log file found"; \
	fi

## 热加载开发模式（需要 air: go install github.com/air-verse/air@latest）
air:
	@AIR=$$(go env GOPATH)/bin/air; \
	if [ ! -f "$$AIR" ]; then \
		echo ">>> Installing air..."; \
		go install github.com/air-verse/air@latest; \
	fi; \
	echo ">>> Starting hot-reload dev mode..."; \
	exec "$$AIR"

clean:
	@rm -f $(BINARY) $(PID_FILE) $(LOG_FILE)
	@rm -rf tmp
	@echo ">>> Cleaned"
