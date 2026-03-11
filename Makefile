.PHONY: build build-gui build-all test lint clean install dev-gui package-mac package-mac-signed sign notarize package-dmg release

BINARY := aisupervisor
BUILD_DIR := ./build
CMD := ./cmd/aisupervisor
CMD_GUI := ./cmd/aisupervisor-gui
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.Version=$(VERSION)

build:
	go build -o $(BUILD_DIR)/$(BINARY) $(CMD)

build-gui:
	cd frontend && npm install && npm run build
	cd $(CMD_GUI) && ~/go/bin/wails build -s -skipbindings -ldflags "$(LDFLAGS)"

build-gui-full:
	cd $(CMD_GUI) && ~/go/bin/wails build -skipbindings

build-all: build build-gui

dev-gui:
	cd $(CMD_GUI) && ~/go/bin/wails dev -skipbindings

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

package-mac:
	cd $(CMD_GUI) && ~/go/bin/wails build -platform darwin/universal -ldflags "$(LDFLAGS)"
	@echo "Built: $(CMD_GUI)/build/bin/aisupervisor-gui.app"

package-mac-signed:
	cd $(CMD_GUI) && ~/go/bin/wails build -platform darwin/universal -ldflags "$(LDFLAGS)"
	codesign --force --deep --sign - $(CMD_GUI)/build/bin/aisupervisor-gui.app
	@echo "Built and signed: $(CMD_GUI)/build/bin/aisupervisor-gui.app"

APP_PATH := $(CMD_GUI)/build/bin/aisupervisor.app
DMG_PATH := build/aisupervisor-$(VERSION).dmg
DEVELOPER_ID ?= $(shell security find-identity -v -p codesigning | grep "Developer ID Application" | head -1 | sed 's/.*"\(.*\)"/\1/')

bundle-deps:
	./scripts/bundle-deps.sh $(APP_PATH)

sign:
	codesign --deep --force --options runtime \
		--sign "Developer ID Application: $(DEVELOPER_ID)" \
		$(APP_PATH)
	@echo "Signed: $(APP_PATH)"

notarize:
	xcrun notarytool submit $(DMG_PATH) \
		--keychain-profile "AC_PASSWORD" --wait
	xcrun stapler staple $(DMG_PATH)
	@echo "Notarized and stapled: $(DMG_PATH)"

package-dmg:
	@mkdir -p build
	create-dmg \
		--volname "AI Supervisor" \
		--window-pos 200 120 \
		--window-size 600 400 \
		--icon-size 100 \
		--app-drop-link 400 200 \
		--icon "aisupervisor.app" 200 200 \
		$(DMG_PATH) \
		$(APP_PATH)
	@echo "DMG created: $(DMG_PATH)"

release: build-gui bundle-deps sign package-dmg notarize
	@echo "Release $(VERSION) complete: $(DMG_PATH)"
