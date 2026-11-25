COMPOSE := docker compose -f deploy/compose.yaml
# Reads KEY from deploy/.env when the value is used.
envval = $(shell grep ^$(1)= deploy/.env | cut -d= -f2)
DEV_DB_URL =postgres://dipwatch:$(call envval,POSTGRES_PASSWORD)@localhost:$(call envval,PG_HOST_PORT)/dipwatch?sslmode=disable

.PHONY: help env dev down logs db test test-go test-ts lint contract check-contract spike sqlc source-smoke refresh agent-eval

help: ## List targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-16s %s\n", $$1, $$2}'

env: deploy/.env ## Create deploy/.env with random secrets if missing

deploy/.env:
	@sed -e "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=$$(openssl rand -hex 16)/" \
	     -e "s/^CORE_INTERNAL_TOKEN=.*/CORE_INTERNAL_TOKEN=$$(openssl rand -hex 32)/" \
	     -e "s/^BETTER_AUTH_SECRET=.*/BETTER_AUTH_SECRET=$$(openssl rand -hex 32)/" \
	     deploy/.env.example > deploy/.env
	@echo "created deploy/.env"

dev: env ## Build and start the full stack at http://localhost:18080
	$(COMPOSE) build
	$(COMPOSE) up -d
	@echo "open http://localhost:$(call envval,HTTP_PORT)"

down: ## Stop the stack (keeps data)
	$(COMPOSE) down

logs: ## Follow logs of all services
	$(COMPOSE) logs -f

db: env ## Start only Postgres, for running tests on the host
	$(COMPOSE) up -d --wait postgres

test: test-go test-ts ## Run all tests

test-go: ## Go tests (uses local Postgres when `make db` is running)
	cd core && DATABASE_URL="$(DEV_DB_URL)" go test ./...

test-ts: ## TypeScript tests
	pnpm test

lint: ## Lint Go and TypeScript
	cd core && go vet ./...
	pnpm lint
	pnpm typecheck

contract: ## Regenerate the OpenAPI contract and the web client types
	cd core && go run ./cmd/core-api openapi > ../contracts/openapi.yaml
	pnpm --filter @dipwatch/web contract

check-contract: contract ## Fail if generated contract files differ from the committed ones
	git diff --exit-code -- contracts web/src/services/schema.d.ts

spike: ## Run the Day 1 source feasibility spike
	cd tools/spike && go run ./sources

sqlc: ## Regenerate typed queries after changing migrations or queries
	cd core && sqlc generate

source-smoke: ## Search one live source, e.g. make source-smoke SOURCE=kakaku Q="RTX 5090"
	cd core && go run ./cmd/source-smoke -source "$(or $(SOURCE),kakaku)" -q "$(or $(Q),RTX 5090)"

agent-eval: ## Run the agent eval with the real LLM (costs money), e.g. make agent-eval NOTE="what changed"
	cd core && go run ./cmd/agent-eval -env ../deploy/.env -report ../docs/harness -note "$(NOTE)" $(ARGS)

refresh: ## Enqueue a refresh in the running stack, e.g. make refresh Q="RTX 5090"
	$(COMPOSE) exec worker worker refresh "$(or $(Q),RTX 5090)"
