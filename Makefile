# Default OS/ARCH values
OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
# Default to static Go binaries so Linux artifacts stay compatible with older glibc baselines.
CGO_ENABLED ?= 0
GO_BUILD_ENV := CGO_ENABLED=$(CGO_ENABLED) GOGC=75 GOOS=$(OS) GOARCH=$(ARCH)
# Skip building the web UI if true
SKIP_WEB ?= false

# Set executable extension based on target OS
EXE_EXT := $(if $(filter windows,$(OS)),.exe,)

.PHONY: tidy build-agent build-hub build-hub-dev build clean lint dev-server dev-ui dev-agent dev-hub dev generate-locales
.DEFAULT_GOAL := build

clean:
	go clean
	rm -rf ./build

lint:
	golangci-lint run

test: export GOEXPERIMENT=synctest
test:
	go test -tags=testing ./...

tidy:
	go mod tidy

build-web-ui:
	@if command -v bun >/dev/null 2>&1; then \
		bun install --cwd ./internal/site && \
		bun run --cwd ./internal/site build; \
	else \
		npm install --prefix ./internal/site && \
		npm run --prefix ./internal/site build; \
	fi

# Conditional .NET build - only for Windows
build-dotnet-conditional:
	@if [ "$(OS)" = "windows" ]; then \
		echo "Building .NET executable for Windows..."; \
		if command -v dotnet >/dev/null 2>&1; then \
			rm -rf ./agent/lhm/bin; \
			dotnet build -c Release ./agent/lhm/aether_lhm.csproj; \
		else \
			echo "Error: dotnet not found. Install .NET SDK to build Windows agent."; \
			exit 1; \
		fi; \
	fi

# Update build-agent to include conditional .NET build
build-agent: tidy build-dotnet-conditional
	$(GO_BUILD_ENV) go build -o ./build/aether-agent_$(OS)_$(ARCH)$(EXE_EXT) -ldflags "-w -s" ./internal/cmd/agent

build-hub: tidy $(if $(filter false,$(SKIP_WEB)),build-web-ui)
	$(GO_BUILD_ENV) go build -o ./build/aether_$(OS)_$(ARCH)$(EXE_EXT) -ldflags "-w -s" ./internal/cmd/hub

build-hub-dev: tidy
	mkdir -p ./internal/site/dist && touch ./internal/site/dist/index.html
	$(GO_BUILD_ENV) go build -tags development -o ./build/aether-dev_$(OS)_$(ARCH)$(EXE_EXT) -ldflags "-w -s" ./internal/cmd/hub

build: build-agent build-hub

generate-locales:
	@if [ ! -f ./internal/site/src/locales/en/en.ts ]; then \
		echo "Generating locales..."; \
		command -v bun >/dev/null 2>&1 && cd ./internal/site && bun install && bun run sync || cd ./internal/site && npm install && npm run sync; \
	fi

dev-server: generate-locales
	@{ set -a; [ -f "./local-dev.env" ] && . "./local-dev.env"; set +a; } && \
	if command -v bun >/dev/null 2>&1; then \
		cd ./internal/site && bun run dev --host 0.0.0.0; \
	else \
		cd ./internal/site && npm run dev --host 0.0.0.0; \
	fi

dev-ui: dev-server

dev-hub: export ENV=dev
dev-hub:
	@{ set -a; [ -f "./local-dev.env" ] && . "./local-dev.env"; set +a; \
	export AETHER_HUB_INGEST_MONITOR_PG_HOST="$${AETHER_HUB_INGEST_MONITOR_PG_HOST:-localhost}"; \
	export AETHER_HUB_INGEST_MONITOR_PG_PORT="$${AETHER_HUB_INGEST_MONITOR_PG_PORT:-5432}"; \
	export AETHER_HUB_INGEST_MONITOR_PG_USER="$${AETHER_HUB_INGEST_MONITOR_PG_USER:-app_user}"; \
	export AETHER_HUB_INGEST_MONITOR_PG_PASSWORD="$${AETHER_HUB_INGEST_MONITOR_PG_PASSWORD:-app_pass}"; \
	export AETHER_HUB_INGEST_MONITOR_PG_DATABASE="$${AETHER_HUB_INGEST_MONITOR_PG_DATABASE:-i3d_multitenant}"; \
	export AETHER_HUB_INGEST_MONITOR_PG_TENANT="$${AETHER_HUB_INGEST_MONITOR_PG_TENANT:-guochuang}"; \
	export AETHER_HUB_INGEST_MONITOR_PG_SSLMODE="$${AETHER_HUB_INGEST_MONITOR_PG_SSLMODE:-disable}"; \
	export AETHER_HUB_DATA_CLEANUP_KEY="$${AETHER_HUB_DATA_CLEANUP_KEY:-0123456789abcdef0123456789abcdef}"; \
	export AETHER_HUB_LICENSE_PRIVATE_KEY_FILE="$${AETHER_HUB_LICENSE_PRIVATE_KEY_FILE:-$(CURDIR)/.hq-license/license_signing_ed25519_private.pem}"; \
	export AETHER_HUB_LICENSE_MODEL_MANIFEST="$${AETHER_HUB_LICENSE_MODEL_MANIFEST:-$(CURDIR)/.hq-license/model_security_manifest.json}"; \
	export AETHER_HUB_LOCAL_AGENT_BIN="$${AETHER_HUB_LOCAL_AGENT_BIN:-$(CURDIR)/build/aether-agent_$(OS)_$(ARCH)$(EXE_EXT)}"; \
	export APP_URL="$${APP_URL:-http://192.168.140.2:19090}"; \
	mkdir -p ./internal/site/dist && touch ./internal/site/dist/index.html; \
	if command -v entr >/dev/null 2>&1; then \
		find ./internal -type f -name '*.go' | entr -r -s "cd ./internal/cmd/hub && go run -tags development . serve --http 0.0.0.0:19090"; \
	else \
		cd ./internal/cmd/hub && go run -tags development . serve --http 0.0.0.0:19090; \
	fi; }

dev-agent:
	@{ set -a; [ -f "./local-dev.env" ] && . "./local-dev.env"; set +a; \
	export AETHER_AGENT_LOG_LEVEL="$${AETHER_AGENT_LOG_LEVEL:-debug}"; \
	export KEY="$${KEY:-ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIp08AUAbaI8tyZkH4eOYEEDQdfrJR3h6aICr3EI8R+v}"; \
	export TOKEN="$${TOKEN:-5d95-71a98ecf34-320b-bacd1f51f}"; \
	export HUB_URL="$${HUB_URL:-http://192.168.140.2:19090}"; \
	if command -v entr >/dev/null 2>&1; then \
		find ./internal/cmd/agent/*.go ./agent/*.go | entr -r go run ./internal/cmd/agent; \
	else \
		go run ./internal/cmd/agent; \
	fi; }
	
build-dotnet:
	@if command -v dotnet >/dev/null 2>&1; then \
		rm -rf ./agent/lhm/bin; \
			dotnet build -c Release ./agent/lhm/aether_lhm.csproj; \
	else \
		echo "dotnet not found"; \
	fi


# KEY="..." make -j dev
dev: dev-server dev-hub dev-agent
