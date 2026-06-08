SHELL := /bin/bash

ROOT_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
FRONTEND_DIR := $(ROOT_DIR)frontend
BIN_DIR := $(ROOT_DIR)bin
HUB_BIN := $(BIN_DIR)/l-ui-hub
AGENT_BIN := $(BIN_DIR)/l-ui-agent
WEB_DIST_DIR := $(ROOT_DIR)hub/web/dist
TMP_DIR := $(ROOT_DIR)tmp
LOCAL_BIN_DIR := $(TMP_DIR)/bin
LOCAL_DB_DIR := $(TMP_DIR)/db
LOCAL_LOG_DIR := $(TMP_DIR)/log
LOCAL_DEV_BIN := $(TMP_DIR)/l-ui-dev-bin
LOCAL_BACKEND_URL := http://127.0.0.1:2053
GO_TEST_PACKAGES = $(shell go list ./... | awk '!/\/frontend\/node_modules\//')

COLOR_RESET := \033[0m
COLOR_BOLD := \033[1m
COLOR_CYAN := \033[36m
COLOR_GREEN := \033[32m
COLOR_YELLOW := \033[33m
COLOR_RED := \033[31m

RELEASE_GOALS := $(filter-out release,$(MAKECMDGOALS))
RELEASE_VERSION ?= $(firstword $(RELEASE_GOALS))
RELEASE_TITLE ?= $(wordlist 2,$(words $(RELEASE_GOALS)),$(RELEASE_GOALS))
RELEASE_FORCE ?= $(if $(FORCE),$(FORCE),0)

.DEFAULT_GOAL := help
.PHONY: help dev dev-local run build build-hub build-agent clean test test-back test-front test-all lint typecheck gen-api gen-zod \
        frontend-install frontend-build frontend-test frontend-lint frontend-typecheck \
        dev-ensure-web-dist release docker-build docker-push fmt smoke-test

help:
	@printf '%b\n' \
		'$(COLOR_BOLD)Available targets:$(COLOR_RESET)' \
		'  $(COLOR_CYAN)make dev$(COLOR_RESET)            Start the local backend first, then Vite' \
		'  $(COLOR_CYAN)make build$(COLOR_RESET)          Build frontend assets and both Go binaries' \
		'  $(COLOR_CYAN)make build-hub$(COLOR_RESET)      Build the hub binary only' \
		'  $(COLOR_CYAN)make build-agent$(COLOR_RESET)    Build the agent binary only' \
		'  $(COLOR_CYAN)make clean$(COLOR_RESET)          Remove local build artifacts' \
		'  $(COLOR_CYAN)make test$(COLOR_RESET)           Run backend + frontend tests' \
		'  $(COLOR_CYAN)make test-back$(COLOR_RESET)      Run repo-wide Go tests' \
		'  $(COLOR_CYAN)make test-front$(COLOR_RESET)     Run frontend Vitest suite' \
		'  $(COLOR_CYAN)make lint$(COLOR_RESET)           Run Go vet and frontend lint' \
		'  $(COLOR_CYAN)make typecheck$(COLOR_RESET)      Run frontend TypeScript typecheck' \
		'  $(COLOR_CYAN)make gen-api$(COLOR_RESET)        Regenerate frontend OpenAPI artifacts' \
		'  $(COLOR_CYAN)make gen-zod$(COLOR_RESET)        Regenerate frontend Zod/types artifacts' \
		'  $(COLOR_CYAN)make fmt$(COLOR_RESET)            Format Go code with gofumpt' \
		'  $(COLOR_CYAN)make docker-build$(COLOR_RESET)   Build Docker image l-ui:latest' \
		'  $(COLOR_CYAN)make docker-push$(COLOR_RESET)    Push Docker image to registry' \
		'  $(COLOR_CYAN)make smoke-test$(COLOR_RESET)     Run basic smoke test (build + health check)' \
		'  $(COLOR_CYAN)make release tag=v0.0.1 title="First release"$(COLOR_RESET)  Create and push a git tag' \
		'  $(COLOR_YELLOW)Use title= for multi-word tag messages$(COLOR_RESET)'

release:
	@set -euo pipefail; \
	version="$(if $(tag),$(tag),$(if $(VERSION),$(VERSION),$(RELEASE_VERSION)))"; \
	title="$(if $(title),$(title),$(if $(TITLE),$(TITLE),$(RELEASE_TITLE)))"; \
	force="$(if $(FORCE),$(FORCE),$(RELEASE_FORCE))"; \
	if [ -z "$$version" ] || [ "$$version" = "release" ]; then \
		printf '%b\n' '$(COLOR_RED)[release]$(COLOR_RESET) tag is required, for example: make release tag=v0.0.1 title="First release"'; \
		exit 1; \
	fi; \
	if [ -z "$$title" ]; then \
		printf '%b\n' '$(COLOR_RED)[release]$(COLOR_RESET) title is required, for example: make release tag=v0.0.1 title="First release"'; \
		exit 1; \
	fi; \
	if [ "$${version#v}" = "$$version" ]; then \
		version="v$$version"; \
	fi; \
	printf '%b\n' '$(COLOR_CYAN)[release]$(COLOR_RESET) creating tag '$$version'...'; \
	if [ "$$force" = "1" ] || [ "$$force" = "true" ]; then \
		git tag -f -a "$$version" -m "$$title"; \
		git push --force origin "$$version"; \
	else \
		git tag -a "$$version" -m "$$title"; \
		git push origin "$$version"; \
	fi

frontend-install:
	@if [ ! -d "$(FRONTEND_DIR)/node_modules/.bin" ]; then \
		npm --prefix "$(FRONTEND_DIR)" ci; \
	fi

frontend-build: frontend-install
	@printf '%b\n' '$(COLOR_CYAN)[frontend-build]$(COLOR_RESET) building frontend assets...'
	@npm --prefix "$(FRONTEND_DIR)" run build
	@printf '%b\n' '$(COLOR_GREEN)[frontend-build]$(COLOR_RESET) frontend assets built'

frontend-test: frontend-install
	@printf '%b\n' '$(COLOR_CYAN)[frontend-test]$(COLOR_RESET) running Vitest...'
	@npm --prefix "$(FRONTEND_DIR)" test
	@printf '%b\n' '$(COLOR_GREEN)[frontend-test]$(COLOR_RESET) Vitest completed'

frontend-lint: frontend-install
	@printf '%b\n' '$(COLOR_CYAN)[frontend-lint]$(COLOR_RESET) running ESLint...'
	@npm --prefix "$(FRONTEND_DIR)" run lint
	@printf '%b\n' '$(COLOR_GREEN)[frontend-lint]$(COLOR_RESET) ESLint completed'

frontend-typecheck: frontend-install
	@printf '%b\n' '$(COLOR_CYAN)[frontend-typecheck]$(COLOR_RESET) running TypeScript check...'
	@npm --prefix "$(FRONTEND_DIR)" run typecheck
	@printf '%b\n' '$(COLOR_GREEN)[frontend-typecheck]$(COLOR_RESET) TypeScript check completed'

gen-api: frontend-install
	@npm --prefix "$(FRONTEND_DIR)" run gen:api

gen-zod: frontend-install
	@npm --prefix "$(FRONTEND_DIR)" run gen:zod

build: frontend-build build-hub build-agent

build-hub:
	@printf '%b\n' '$(COLOR_CYAN)[build-hub]$(COLOR_RESET) building hub binary...'
	@mkdir -p "$(BIN_DIR)"
	@go build -buildvcs=false -o "$(HUB_BIN)" ./hub
	@printf '%b\n' '$(COLOR_GREEN)[build-hub]$(COLOR_RESET) hub build complete'

build-agent:
	@printf '%b\n' '$(COLOR_CYAN)[build-agent]$(COLOR_RESET) building agent binary...'
	@mkdir -p "$(BIN_DIR)"
	@go build -buildvcs=false -o "$(AGENT_BIN)" ./agent
	@printf '%b\n' '$(COLOR_GREEN)[build-agent]$(COLOR_RESET) agent build complete'

clean:
	@rm -rf "$(BIN_DIR)" "$(WEB_DIST_DIR)" "$(TMP_DIR)"

test: test-back test-front

test-back: dev-ensure-web-dist
	@printf '%b\n' '$(COLOR_CYAN)[test-back]$(COLOR_RESET) running Go test suite...'
	@go test $(GO_TEST_PACKAGES)
	@printf '%b\n' '$(COLOR_GREEN)[test-back]$(COLOR_RESET) Go tests completed'

test-front: frontend-test

test-all: test-back test-front lint typecheck

lint: dev-ensure-web-dist frontend-lint
	@printf '%b\n' '$(COLOR_CYAN)[lint]$(COLOR_RESET) running Go vet...'
	@go vet $(GO_TEST_PACKAGES)
	@printf '%b\n' '$(COLOR_GREEN)[lint]$(COLOR_RESET) Go vet completed'

typecheck: dev-ensure-web-dist frontend-typecheck

dev-ensure-web-dist:
	@mkdir -p "$(WEB_DIST_DIR)/assets"
	@printf '%s\n' '<!doctype html>' '<html>' '<head></head>' '<body></body>' '</html>' > "$(WEB_DIST_DIR)/index.html"
	@cp "$(WEB_DIST_DIR)/index.html" "$(WEB_DIST_DIR)/login.html"
	@cp "$(WEB_DIST_DIR)/index.html" "$(WEB_DIST_DIR)/subpage.html"
	@printf '%s\n' '{"openapi":"3.0.0","info":{"title":"l-ui","version":"0.0.0"},"paths":{}}' > "$(WEB_DIST_DIR)/openapi.json"


dev:
	@set -euo pipefail; \
	mkdir -p "$(LOCAL_BIN_DIR)" "$(LOCAL_DB_DIR)" "$(LOCAL_LOG_DIR)"; \
	cp -f "$(ROOT_DIR)hub/web/service/config.json" "$(LOCAL_BIN_DIR)/config.json"; \
	frontend_log="$(LOCAL_LOG_DIR)/frontend.log"; \
	backend_log="$(LOCAL_LOG_DIR)/backend.log"; \
	printf '%b\n' '$(COLOR_CYAN)[dev]$(COLOR_RESET) building frontend...'; \
	npm --prefix "$(FRONTEND_DIR)" run build >"$(LOCAL_LOG_DIR)/frontend-build.log" 2>&1; \
	printf '%b\n' '$(COLOR_CYAN)[dev]$(COLOR_RESET) building backend...'; \
	go build -buildvcs=false -o "$(LOCAL_DEV_BIN)" ./hub >"$(LOCAL_LOG_DIR)/backend-build.log" 2>&1; \
	> "$$backend_log"; \
	> "$$frontend_log"; \
	frontend_pid=""; \
	backend_pid=""; \
	cleanup() { \
		if [ -n "$$frontend_pid" ] && kill -0 "$$frontend_pid" >/dev/null 2>&1; then \
			kill -- "-$$frontend_pid" >/dev/null 2>&1 || true; \
			wait "$$frontend_pid" >/dev/null 2>&1 || true; \
		fi; \
		if [ -n "$$backend_pid" ] && kill -0 "$$backend_pid" >/dev/null 2>&1; then \
			kill -- "-$$backend_pid" >/dev/null 2>&1 || true; \
			wait "$$backend_pid" >/dev/null 2>&1 || true; \
		fi; \
	}; \
	trap cleanup INT TERM EXIT; \
	printf '%b\n' '$(COLOR_CYAN)[dev]$(COLOR_RESET) starting backend...'; \
	LUI_DEBUG=true LUI_BIN_FOLDER="$(LOCAL_BIN_DIR)" LUI_DB_FOLDER="$(LOCAL_DB_DIR)" LUI_LOG_FOLDER="$(LOCAL_LOG_DIR)" "$(LOCAL_DEV_BIN)" run >>"$$backend_log" 2>&1 & backend_pid=$$!; \
	for _ in $$(seq 1 120); do \
		if curl -fsS "$(LOCAL_BACKEND_URL)" >/dev/null 2>&1; then \
			break; \
		fi; \
		if ! kill -0 "$$backend_pid" >/dev/null 2>&1; then \
			printf '%b\n' '$(COLOR_RED)[dev]$(COLOR_RESET) backend exited early -- check $$backend_log'; \
			exit 1; \
		fi; \
		sleep 1; \
	done; \
	if ! curl -fsS "$(LOCAL_BACKEND_URL)" >/dev/null 2>&1; then \
		printf '%b\n' '$(COLOR_RED)[dev]$(COLOR_RESET) backend never became ready -- check $$backend_log'; \
		exit 1; \
	fi; \
	printf '%b\n' '  $(COLOR_GREEN)[ok]$(COLOR_RESET) backend ready at http://127.0.0.1:2053'; \
	printf '%b\n' '$(COLOR_CYAN)[dev]$(COLOR_RESET) starting frontend...'; \
	LUI_DB_FOLDER="$(LOCAL_DB_DIR)" LUI_VITE_ALLOWED_HOSTS=any LUI_BACKEND_TARGET="$(LOCAL_BACKEND_URL)" npm --prefix "$(FRONTEND_DIR)" run dev -- --host 0.0.0.0 >>"$$frontend_log" 2>&1 & frontend_pid=$$!; \
	for _ in $$(seq 1 120); do \
		if curl -fsS "http://127.0.0.1:5173/" >/dev/null 2>&1; then \
			break; \
		fi; \
		if ! kill -0 "$$frontend_pid" >/dev/null 2>&1; then \
			printf '%b\n' '$(COLOR_RED)[dev]$(COLOR_RESET) frontend exited early -- check $$frontend_log'; \
			exit 1; \
		fi; \
		sleep 1; \
	done; \
	if ! curl -fsS "http://127.0.0.1:5173/" >/dev/null 2>&1; then \
		printf '%b\n' '$(COLOR_RED)[dev]$(COLOR_RESET) frontend never became ready -- check $$frontend_log'; \
		exit 1; \
	fi; \
	printf '%b\n' '  $(COLOR_GREEN)[ok]$(COLOR_RESET) frontend ready at http://localhost:5173'; \
	printf '%b\n' ''; \
	printf '%b\n' '$(COLOR_BOLD)============================================================$(COLOR_RESET)'; \
	printf '%b\n' '  Backend:  http://127.0.0.1:2053'; \
	printf '%b\n' '  Frontend: http://localhost:5173'; \
	printf '%b\n' '  Login:    admin / admin'; \
	printf '%b\n' '  Logs:     $(LOCAL_LOG_DIR)/'; \
	printf '%b\n' '$(COLOR_BOLD)============================================================$(COLOR_RESET)'; \
	wait "$$frontend_pid" "$$backend_pid" || true

dev-local: dev

run: dev

docker-build:
	docker build -t l-ui:latest .

docker-push: docker-build
	docker push l-ui:latest

fmt:
	gofumpt -w .

smoke-test: build
	@echo "Starting smoke test..."
	@mkdir -p tmp
	@./bin/l-ui-hub run &
	@sleep 2
	@curl -f http://localhost:2053/healthz || (echo "Health check failed"; exit 1)
	@kill $$! || true
	@echo "Smoke test passed"
