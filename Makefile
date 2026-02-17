BIN := ./imagegen
PKG_CONFIG_PATH := $(HOME)/.nix-profile/lib/pkgconfig:$(PKG_CONFIG_PATH)
export PKG_CONFIG_PATH

.PHONY: install build

install:
	@command -v pkg-config >/dev/null 2>&1 || (echo "missing pkg-config; install with: brew install pkg-config" && exit 1)
	go mod download

build: install
	CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries" go build -o $(BIN) ./cmd/imagegen
