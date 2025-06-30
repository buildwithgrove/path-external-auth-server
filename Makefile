########################
### Makefile Helpers ###
########################

.PHONY: list
list: ## List all make targets
	@${MAKE} -pRrn : -f $(MAKEFILE_LIST) 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | egrep -v -e '^[^[:alnum:]]' -e '^$@$$' | sort

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

###############################
### Mock Generation Targets ###
###############################

.PHONY: gen_mocks
gen_mocks: ## Generates the mocks for the project
	mockgen -source=./store/data_source.go -destination=./store/data_source_mock_test.go -package=store

#############################
### SQL Generator Targets ###
#############################

.PHONY: grove_gen_sqlc
grove_gen_sqlc: ## Generates the SQLC code for Grove's portal schema
	sqlc generate -f ./postgres/grove/sqlc/sqlc.yaml
