BIN := ./imagegen
WEB_BIN := ./imagegen-web
WEB_DIR := ./web
ADDR ?= :8080
DATA_DIR ?= $(HOME)/.imagegen
PKG_CONFIG_PATH := $(HOME)/.nix-profile/lib/pkgconfig:$(PKG_CONFIG_PATH)
export PKG_CONFIG_PATH

.PHONY: install web-install web-lint web-typecheck web-build build dev

install:
	@command -v pkg-config >/dev/null 2>&1 || (echo "missing pkg-config; install with: brew install pkg-config" && exit 1)
	go mod download

web-install:
	@command -v npm >/dev/null 2>&1 || (echo "missing npm; install Node.js/npm to build web assets" && exit 1)
	npm --prefix $(WEB_DIR) install

web-build: web-install
	npm --prefix $(WEB_DIR) run build

web-typecheck: web-install
	npm --prefix $(WEB_DIR) run typecheck

web-lint: web-install
	npm --prefix $(WEB_DIR) run lint

build: install web-lint web-typecheck web-build
	CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries" go build -o $(BIN) ./cmd/imagegen
	CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries" go build -o $(WEB_BIN) ./cmd/imagegen-web

dev: build
	$(WEB_BIN) -addr $(ADDR) -data-dir $(DATA_DIR)
