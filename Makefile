# Avoid hardcoding /bin/bash (e.g., NixOS).
# Default make shell (/bin/sh) is sufficient here.

# Load .env if present and export vars to recipes.
ifneq (,$(wildcard .env))
include .env
export
endif

BIN := bin/mcp-sandboxd
PKG := ./cmd/mcp-sandboxd
DEV_PKG := ./cmd/devwatch

# Explicit dev ports to avoid conflicts.
DEV_PORT ?= 8080
AGENT_PORT ?= 18080

DOCKER ?= docker
SERVER_IMAGE ?= mcp-sandboxd:dev
SANDBOX_IMAGE_DEV ?= mcp-sandboxd-sandbox:dev
DIND_NAME ?= mcp-sandboxd-dind
DIND_PORT ?= 23750
NET ?= mcp-sandboxd-net

# Default to CGO disabled to avoid requiring a C toolchain.
CGO_ENABLED ?= 0
GO := env CGO_ENABLED=$(CGO_ENABLED) go

.PHONY: build run dev dev--agent docker-build docker-run docker-build-sandbox docker-run-sandbox fmt test test-watch clean

build:
	@mkdir -p bin
	$(GO) build -o $(BIN) $(PKG)

run:
	: "$${SANDBOX_IMAGE:?Set SANDBOX_IMAGE (sandbox container image)}"
	PORT=$(DEV_PORT) MCP_PATH=$${MCP_PATH:-/mcp} ARTIFACTS_DIR=$${ARTIFACTS_DIR:-./data/artifacts} \
	$(GO) run $(PKG)

dev:
	: "$${SANDBOX_IMAGE:?Set SANDBOX_IMAGE (sandbox container image)}"
	PORT=$(DEV_PORT) MCP_PATH=$${MCP_PATH:-/mcp} ARTIFACTS_DIR=$${ARTIFACTS_DIR:-./data/artifacts} DEBUG_UI_ENABLED=$${DEBUG_UI_ENABLED:-true} \
	$(GO) run $(DEV_PKG) -pkg $(PKG) -out $(BIN)

dev--agent:
	: "$${SANDBOX_IMAGE:?Set SANDBOX_IMAGE (sandbox container image)}"
	PORT=$(AGENT_PORT) MCP_PATH=$${MCP_PATH:-/mcp} ARTIFACTS_DIR=$${ARTIFACTS_DIR:-./data/agent/artifacts} DEBUG_UI_ENABLED=$${DEBUG_UI_ENABLED:-true} \
	$(GO) run $(DEV_PKG) -pkg $(PKG) -out $(BIN)

docker-build:
	$(DOCKER) build -t $(SERVER_IMAGE) -f docker/server.Dockerfile .

docker-build-sandbox:
	$(DOCKER) build -t $(SANDBOX_IMAGE_DEV) -f docker/sandbox.Dockerfile .

docker-run-sandbox: docker-build-sandbox
	$(DOCKER) run --rm -it $(SANDBOX_IMAGE_DEV) bash

docker-run: docker-build
	: "$${SANDBOX_IMAGE:=$(SANDBOX_IMAGE_DEV)}"
	@mkdir -p data/artifacts
	$(DOCKER) network inspect $(NET) >/dev/null 2>&1 || $(DOCKER) network create $(NET)
	$(DOCKER) rm -f $(DIND_NAME) >/dev/null 2>&1 || true
	$(DOCKER) run -d --privileged --name $(DIND_NAME) --network $(NET) -p 127.0.0.1:$(DIND_PORT):2375 $(DIND_IMAGE) --host=tcp://0.0.0.0:2375
	@echo "Waiting for DinD...";
	@for i in 1 2 3 4 5 6 7 8 9 10; do \
		DOCKER_HOST=tcp://127.0.0.1:$(DIND_PORT) $(DOCKER) info >/dev/null 2>&1 && break; \
		sleep 0.5; \
	done
	@echo "Building sandbox image inside DinD: $${SANDBOX_IMAGE}";
	DOCKER_HOST=tcp://127.0.0.1:$(DIND_PORT) $(DOCKER) build -t $${SANDBOX_IMAGE} -f docker/sandbox.Dockerfile .
	$(DOCKER) rm -f mcp-sandboxd >/dev/null 2>&1 || true
	$(DOCKER) run --rm -it \
		--name mcp-sandboxd \
		--network $(NET) \
		-p $${PORT:-8080}:8080 \
		-e PORT=8080 \
		-e MCP_PATH=$${MCP_PATH:-/mcp} \
		-e DEBUG_UI_ENABLED=$${DEBUG_UI_ENABLED:-true} \
		-e SANDBOX_IMAGE=$${SANDBOX_IMAGE} \
		-e DOCKER_HOST=tcp://$(DIND_NAME):2375 \
		-e ARTIFACTS_DIR=/data/artifacts \
		-v "$$PWD/data/artifacts:/data/artifacts" \
		$(SERVER_IMAGE)

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

test-watch:
	watch -c -n 10 make test

clean:
	rm -rf bin data
