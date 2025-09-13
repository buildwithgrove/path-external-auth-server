################################
### Local Development Stack  ###
################################

.PHONY: metrics-up
metrics-up: ## Start local Prometheus and Grafana stack for metrics development
	cd grafana/local && docker compose up -d

.PHONY: metrics-down
metrics-down: ## Stop local metrics stack (preserves data)
	cd grafana/local && docker compose down

.PHONY: metrics-clean
metrics-clean: ## Stop local metrics stack and remove all data volumes
	cd grafana/local && docker compose down -v

############################
### Load Testing Targets ###
############################

.PHONY: load-test
load-test: ## Run load test with default parameters (10k requests, 75% success rate)
	cd grafana/local && chmod +x load_test.sh && ./load_test.sh

.PHONY: load-test-custom
load-test-custom: ## Run load test with custom parameters (TOTAL_REQUESTS, SUCCESS_RATE)
	cd grafana/local && chmod +x load_test.sh && TOTAL_REQUESTS=${TOTAL_REQUESTS:-1000} SUCCESS_RATE=${SUCCESS_RATE:-75} ./load_test.sh
