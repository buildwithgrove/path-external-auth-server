########################
### Makefile Helpers ###
########################

.PHONY: help
.DEFAULT_GOAL := help
help: ## Prints all the targets in all the Makefiles
	@grep -h -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-60s\033[0m %s\n", $$1, $$2}'

####################
### Test Targets ###
####################

.PHONY: test
test: ## Runs all tests
	go test ./... -count=1

.PHONY: test_verbose
test_verbose: ## Runs all tests with verbose output enabled
	go test -v ./... -count=1

.PHONY: test_unit
test_unit: ## Runs unit tests only (excludes Postgres Docker integration tests)
	go test ./... -short -count=1

.PHONY: go_lint
go_lint: ## Run all go linters
	golangci-lint run --timeout 5m --build-tags test --fix

###############################
### Mock Generation Targets ###
###############################

.PHONY: gen_mocks
gen_mocks: ## Generates the mocks for the project
	mockgen -source=./auth/auth_handler.go -destination=./auth/auth_handler_mock_test.go -package=auth
	mockgen -source=./ratelimit/ratelimit_store.go -destination=./ratelimit/ratelimit_store_mock_test.go -package=ratelimit
	mockgen -source=./store/data_source.go -destination=./store/data_source_mock_test.go -package=store

#############################
### SQL Generator Targets ###
#############################

.PHONY: grove_gen_sqlc
grove_gen_sqlc: ## Generates the SQLC code for Grove's portal schema
	sqlc generate -f ./postgres/grove/sqlc/sqlc.yaml

#############################
### Development Targets   ###
#############################

.PHONY: peas_run
peas_run: load_env peas_build ## Run the PEAS binary as a standalone binary
	@echo "🚀 Starting PEAS server..."
	@if [ -f .env ]; then \
		export $$(grep -v '^#' .env | xargs) && (cd bin; ./peas) || true; \
	else \
		(cd bin; ./peas) || true; \
	fi

.PHONY: peas_build
peas_build: ## Build the PEAS binary locally (does not run anything)
	go build -o bin/peas .

.PHONY: load_env
load_env: ## Load and validate environment variables from .env file
	@if [ -f .env ]; then \
		echo "🔧 Loading environment variables from .env file"; \
		echo "📋 Environment variables loaded:"; \
		grep -v '^#' .env | grep -v '^$$' | while IFS='=' read -r key value; do \
			echo "  ✓ $$key"; \
		done; \
	else \
		echo "⚠️  No .env file found in repo root"; \
		echo "💡 Create a .env file with required variables:"; \
		echo "   POSTGRES_CONNECTION_STRING=postgresql://..."; \
		echo "   GCP_PROJECT_ID=your-project-id"; \
		echo "   PORT=10001"; \
		echo "   LOGGER_LEVEL=info"; \
		echo "   REFRESH_INTERVAL=30s"; \
	fi

.PHONY: get_portal_app_auth_status
get_portal_app_auth_status: ## Test auth/rate limit status for a Portal App ID (requires PORTAL_APP_ID, optional API_KEY)
	@if [ -z "$(PORTAL_APP_ID)" ]; then \
		echo "❌ Error: PORTAL_APP_ID is required"; \
		echo ""; \
		echo "Usage:"; \
		echo "  make get_portal_app_auth_status PORTAL_APP_ID=your-app-id [API_KEY=your-api-key]"; \
		echo ""; \
		echo "Examples:"; \
		echo "  # Test with Portal App ID in path (no auth required)"; \
		echo "  make get_portal_app_auth_status PORTAL_APP_ID=1a2b3c4d"; \
		echo ""; \
		echo "  # Test with Portal App ID in header + API key"; \
		echo "  make get_portal_app_auth_status PORTAL_APP_ID=1a2b3c4d API_KEY=4c352139ec5ca9288126300271d08867"; \
		echo ""; \
		echo "  # Test Portal App ID in path + API key in header"; \
		echo "  make get_portal_app_auth_status PORTAL_APP_ID=1a2b3c4d API_KEY=4c352139ec5ca9288126300271d08867"; \
		exit 1; \
	fi
	@echo "🫛 Testing PEAS auth for Portal App ID: $(PORTAL_APP_ID)"
	@if [ -n "$(API_KEY)" ]; then \
		echo "🔑 Using API Key: $(API_KEY)"; \
		grpcurl -plaintext \
			-d '{ \
				"attributes": { \
					"request": { \
						"http": { \
							"method": "GET", \
							"path": "/v1/$(PORTAL_APP_ID)", \
							"headers": { \
								"authorization": "$(API_KEY)" \
							} \
						} \
					} \
				} \
			}' \
			localhost:10001 \
			envoy.service.auth.v3.Authorization/Check | jq; \
	else \
		echo "📝 No API key provided - testing without authorization header"; \
		grpcurl -plaintext \
			-d '{ \
				"attributes": { \
					"request": { \
						"http": { \
							"method": "GET", \
							"path": "/v1/$(PORTAL_APP_ID)", \
							"headers": {} \
						} \
					} \
				} \
			}' \
			localhost:10001 \
			envoy.service.auth.v3.Authorization/Check | jq; \
	fi

include makefiles/local.mk
