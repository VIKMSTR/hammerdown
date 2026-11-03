# mkup — Markdown editor (Go + Fyne)
#
#   make            build ./mkup for this machine
#   make run        build and start (FILE=notes.md to open a file)
#   make check      format check + vet + tests
#   make package    native app bundle (mkup.app on macOS, tar.xz on Linux, .exe on Windows)
#   make help       list all targets

APP      := mkup
# Name, ID, icon and version come from FyneApp.toml (the fyne tool bumps
# Build there on every release package).

GO       ?= go
GOOS     := $(shell $(GO) env GOOS)
BIN_DIR  := $(CURDIR)/bin
FYNE     := $(BIN_DIR)/fyne
FYNE_TOOLS_VERSION := v1.7.4

# Fyne needs cgo (OpenGL + native windowing).
export CGO_ENABLED := 1

# The macOS linker warns about the duplicate -lobjc that cgo packages add;
# it is harmless, so silence it.
ifeq ($(GOOS),darwin)
export CGO_LDFLAGS := $(CGO_LDFLAGS) -Wl,-no_warn_duplicate_libraries
endif

ifeq ($(GOOS),windows)
EXE := .exe
# Hide the console window for the GUI binary.
GUI_LDFLAGS := -H=windowsgui
endif

LDFLAGS  := -s -w $(GUI_LDFLAGS)
SOURCES  := $(shell find . -name '*.go' -not -path './bin/*') go.mod go.sum \
            internal/ui/reference.md

.DEFAULT_GOAL := build
.PHONY: build run test test-race vet fmt fmt-check check package tools clean help

build: $(APP)$(EXE) ## Build the optimised binary for this machine

$(APP)$(EXE): $(SOURCES)
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $@ .

run: build ## Build and run (FILE=path.md to open a document)
	./$(APP)$(EXE) $(FILE)

test: ## Run all tests
	$(GO) test ./...

test-race: ## Run all tests with the race detector
	$(GO) test -race ./...

vet: ## Run go vet
	$(GO) vet ./...

fmt: ## Format the code
	gofmt -w $(shell find . -name '*.go' -not -path './bin/*')

fmt-check: ## Fail if any file needs formatting
	@out=$$(gofmt -l $$(find . -name '*.go' -not -path './bin/*')); \
	if [ -n "$$out" ]; then echo "Needs gofmt:"; echo "$$out"; exit 1; fi

check: fmt-check vet test ## Format check, vet and tests

tools: $(FYNE) ## Install the fyne packaging tool into ./bin

$(FYNE):
	GOBIN=$(BIN_DIR) $(GO) install fyne.io/tools/cmd/fyne@$(FYNE_TOOLS_VERSION)

package: $(FYNE) ## Build a native app bundle for this OS (mkup.app on macOS)
	rm -rf $(APP).app
	$(GO) build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(APP)$(EXE) .
	$(FYNE) package --os $(GOOS) --release --exe $(BIN_DIR)/$(APP)$(EXE)

clean: ## Remove build outputs
	rm -rf $(APP) $(APP).exe $(APP).app $(APP).tar.xz $(BIN_DIR)
	$(GO) clean -testcache

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
