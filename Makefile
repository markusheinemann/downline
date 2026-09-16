# This makefile is heavily inspired from
# https://mohitkhare.com/blog/go-makefile/

export GO111MODULE=on

# Optional colors to beautify output
GREEN  := $(shell tput -Txterm setaf 2)
YELLOW := $(shell tput -Txterm setaf 3)
WHITE  := $(shell tput -Txterm setaf 7)
CYAN   := $(shell tput -Txterm setaf 6)
RESET  := $(shell tput -Txterm sgr0)

# Minimum total test coverage in percent, excluding cmd/
COVERAGE_MIN ?= 80

## Quality
check-quality: ## runs code quality checks
	make fmt
	make vet

fmt: ## run formatter
	go fmt ./...

vet: ## run built-in static analysis tool
	go vet ./...

tidy: ## runs tidy to fix go.mod dependencies
	go mod tidy

## Testing
test: ## runs tests and create generates coverage report
	make tidy
	go test -v -race -timeout 10m ./... -coverprofile=coverage.out -json > report.json

coverage-check: ## checks if the minimum code coverage is given
	@grep -v '/cmd/' coverage.out > coverage.filtered.out
	@total=$$(go tool cover -func=coverage.filtered.out | awk '/^total:/ { sub("%", "", $$3); print $$3 }'); \
	echo "Total coverage (excluding cmd/): $$total% (minimum: $(COVERAGE_MIN)%)"; \
	awk -v total="$$total" -v min="$(COVERAGE_MIN)" 'BEGIN { exit !(total >= min) }' || { echo "Coverage is below $(COVERAGE_MIN)%"; exit 1; }

coverage: ## displays test coverage report in html mode
	make test
	go tool cover -html=coverage.out

## Build
build: ## build all binaries
	make build-collector

build-collector: ## build the go application
	mkdir -p out/
	go build -o ./out/collector ./cmd/collector
	@echo "Build for collector passed"

run-collector: ## runs the collector binary
	make build-collector
	chmod +x ./out/collector
	./out/collector

clean: ## cleans binary and other generated files
	go clean
	rm -rf out/
	rm -f coverage*.out

## Help
help: ## Show this help.
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target>${RESET}'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
        if (/^[a-zA-Z_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
        else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
        }' $(MAKEFILE_LIST)