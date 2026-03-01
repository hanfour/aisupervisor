.PHONY: build build-gui build-all test lint clean install dev-gui

BINARY := aisupervisor
BUILD_DIR := ./build
CMD := ./cmd/aisupervisor
CMD_GUI := ./cmd/aisupervisor-gui

build:
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

build-gui:
	cd frontend && npm install && npm run build
	go build -o $(BUILD_DIR)/$(BINARY)-gui $(CMD_GUI)

build-all: build build-gui

dev-gui:
	wails dev

install:
	go install $(CMD)

test:
	go test ./... -v

test-short:
	go test ./... -short

test-detector:
	go test ./internal/detector/... -v

test-supervisor:
	go test ./internal/supervisor/... -v

test-group:
	go test ./internal/group/... -v

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)

run-headless:
	go run $(CMD) monitor --session=$(SESSION)

run-tui:
	go run $(CMD) monitor --tui

run-dry:
	go run $(CMD) monitor --dry-run --session=$(SESSION)

config-init:
	go run $(CMD) config init

config-show:
	go run $(CMD) config show
