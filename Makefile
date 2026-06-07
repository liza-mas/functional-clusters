BINARY     ?= functional-clusters
BUILD_DIR  ?= build
CMD        := ./cmd/functional-clusters
GO         ?= go
GO_CACHE_HOME ?= $(HOME)/.cache/go
GOMODCACHE ?= $(GO_CACHE_HOME)/pkg/mod
GOCACHE ?= $(GO_CACHE_HOME)/build
INSTALL_DIR ?= $(HOME)/.local/bin
SOURCE_REF ?= local
SOURCE_REVISION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION    ?= source:$(SOURCE_REF):$(SOURCE_REVISION)
LDFLAGS    := -ldflags "-X github.com/liza-mas/functional-clusters/internal/cli.Version=$(VERSION)"
GOENV      := GOPATH="$(GO_CACHE_HOME)" GOMODCACHE="$(GOMODCACHE)" GOCACHE="$(GOCACHE)"

.PHONY: install build run test clean

install: build
	@dest="$(INSTALL_DIR)/$(BINARY)"; \
	mkdir -p "$(INSTALL_DIR)" || { printf 'INSTALL_DIR is not usable: %s\n' "$(INSTALL_DIR)" >&2; exit 1; }; \
	cp "$(BUILD_DIR)/$(BINARY)" "$$dest" || { printf 'INSTALL_DIR is not usable: %s\n' "$(INSTALL_DIR)" >&2; exit 1; }; \
	chmod 0755 "$$dest" || { printf 'INSTALL_DIR is not usable: %s\n' "$(INSTALL_DIR)" >&2; exit 1; }; \
	if [ ! -x "$$dest" ]; then \
		printf 'installed functional-clusters is not executable: %s\n' "$$dest" >&2; \
		exit 1; \
	fi; \
	if ! version_output=$$("$$dest" --version 2>&1); then \
		printf 'installed functional-clusters at %s failed --version\n' "$$dest" >&2; \
		exit 1; \
	fi; \
	case "$$version_output" in \
	*"$(VERSION)"* ) ;; \
	*) printf 'installed functional-clusters at %s did not report %s\n' "$$dest" "$(VERSION)" >&2; exit 1 ;; \
	esac; \
	printf 'Installed functional-clusters %s to %s\n' "$(VERSION)" "$$dest"

build:
	@mkdir -p "$(BUILD_DIR)"
	$(GOENV) $(GO) build $(LDFLAGS) -o "$(BUILD_DIR)/$(BINARY)" $(CMD)

run: build
	./$(BUILD_DIR)/$(BINARY)

test:
	$(GOENV) $(GO) test ./...

clean:
	rm -rf "$(BUILD_DIR)" dist/
