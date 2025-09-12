
.PHONY: metrics-up
metrics-up:
	cd grafana/local && docker compose up -d

.PHONY: metrics-down
metrics-down:
	cd grafana/local && docker compose down

.PHONY: metrics-clean
metrics-clean:
	cd grafana/local && docker compose down -v

.PHONY: load-test
load-test:
	cd grafana/local && chmod +x load_test.sh && ./load_test.sh

.PHONY: load-test-custom
load-test-custom:
	cd grafana/local && chmod +x load_test.sh && TOTAL_REQUESTS=${TOTAL_REQUESTS:-1000} SUCCESS_RATE=${SUCCESS_RATE:-75} ./load_test.sh
