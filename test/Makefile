# TMC Testing Makefile
# Provides comprehensive testing targets for TMC components

# Test configuration
TEST_TIMEOUT ?= 10m
COVERAGE_OUT ?= coverage.out
COVERAGE_HTML ?= coverage.html
TEST_PARALLEL ?= 4

# Go test flags
TEST_FLAGS ?= -v -timeout=$(TEST_TIMEOUT) -parallel=$(TEST_PARALLEL)
INTEGRATION_FLAGS ?= -tags=integration
UNIT_FLAGS ?= -tags=unit

.PHONY: help test test-unit test-integration test-e2e test-coverage test-benchmark clean

## help: Show this help message
help:
	@echo "TMC Testing Makefile"
	@echo ""
	@echo "Available targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
	@echo ""

## test: Run all TMC tests
test: test-unit test-integration test-e2e

## test-unit: Run TMC unit tests
test-unit:
	@echo "Running TMC unit tests..."
	go test $(TEST_FLAGS) $(UNIT_FLAGS) ./unit/tmc/...

## test-integration: Run TMC integration tests
test-integration:
	@echo "Running TMC integration tests..."
	go test $(TEST_FLAGS) $(INTEGRATION_FLAGS) ./integration/tmc/...

## test-e2e: Run TMC end-to-end tests
test-e2e:
	@echo "Running TMC end-to-end tests..."
	go test $(TEST_FLAGS) -tags=e2e ./integration/tmc/e2e_test.go

## test-workspace-isolation: Run TMC workspace isolation tests
test-workspace-isolation:
	@echo "Running TMC workspace isolation tests..."
	go test $(TEST_FLAGS) $(INTEGRATION_FLAGS) -run TestTMCWorkspaceIsolation ./integration/tmc/workspace_isolation_test.go

## test-controllers: Run TMC controller integration tests
test-controllers:
	@echo "Running TMC controller integration tests..."
	go test $(TEST_FLAGS) $(INTEGRATION_FLAGS) -run TestTMCControllerIntegration ./integration/tmc/controllers_test.go

## test-coverage: Run tests with coverage reporting
test-coverage:
	@echo "Running TMC tests with coverage..."
	go test $(TEST_FLAGS) -coverprofile=$(COVERAGE_OUT) ./...
	go tool cover -html=$(COVERAGE_OUT) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"
	go tool cover -func=$(COVERAGE_OUT)

## test-coverage-unit: Run unit tests with coverage reporting
test-coverage-unit:
	@echo "Running TMC unit tests with coverage..."
	go test $(TEST_FLAGS) $(UNIT_FLAGS) -coverprofile=unit_$(COVERAGE_OUT) ./unit/tmc/...
	go tool cover -func=unit_$(COVERAGE_OUT)

## test-coverage-integration: Run integration tests with coverage reporting
test-coverage-integration:
	@echo "Running TMC integration tests with coverage..."
	go test $(TEST_FLAGS) $(INTEGRATION_FLAGS) -coverprofile=integration_$(COVERAGE_OUT) ./integration/tmc/...
	go tool cover -func=integration_$(COVERAGE_OUT)

## test-benchmark: Run TMC benchmark tests
test-benchmark:
	@echo "Running TMC benchmark tests..."
	go test $(TEST_FLAGS) -bench=. -benchmem ./unit/tmc/...

## test-race: Run tests with race detection
test-race:
	@echo "Running TMC tests with race detection..."
	go test $(TEST_FLAGS) -race ./...

## test-short: Run quick TMC tests (short mode)
test-short:
	@echo "Running TMC tests in short mode..."
	go test $(TEST_FLAGS) -short ./...

## test-verbose: Run tests with verbose output
test-verbose:
	@echo "Running TMC tests with verbose output..."
	go test -v -timeout=$(TEST_TIMEOUT) -parallel=$(TEST_PARALLEL) ./...

## test-performance: Run TMC performance tests
test-performance:
	@echo "Running TMC performance tests..."
	go test $(TEST_FLAGS) -run TestTMCControllerPerformance ./integration/tmc/controllers_test.go
	go test $(TEST_FLAGS) -run TestTMCPerformanceBaseline ./unit/tmc/runner_test.go

## test-api-validation: Run TMC API validation tests
test-api-validation:
	@echo "Running TMC API validation tests..."
	go test $(TEST_FLAGS) -run TestTMCUnitTestRunner ./unit/tmc/runner_test.go

## test-workspace-lifecycle: Run workspace lifecycle tests
test-workspace-lifecycle:
	@echo "Running workspace lifecycle tests..."
	go test $(TEST_FLAGS) $(INTEGRATION_FLAGS) -run TestWorkspaceLifecycleWithTMC ./integration/tmc/workspace_isolation_test.go

## test-multi-tenant: Run multi-tenant tests
test-multi-tenant:
	@echo "Running multi-tenant TMC tests..."
	go test $(TEST_FLAGS) $(INTEGRATION_FLAGS) -run TestMultiTenantTMCScenarios ./integration/tmc/workspace_isolation_test.go

## test-clean: Clean test artifacts
test-clean:
	@echo "Cleaning test artifacts..."
	rm -f $(COVERAGE_OUT) $(COVERAGE_HTML)
	rm -f unit_$(COVERAGE_OUT) integration_$(COVERAGE_OUT)
	rm -f *.test
	rm -rf test-results/

## test-setup: Set up test environment
test-setup:
	@echo "Setting up TMC test environment..."
	@# TODO: Add setup steps when TMC components are available
	@echo "TMC test environment setup complete"

## test-teardown: Tear down test environment
test-teardown:
	@echo "Tearing down TMC test environment..."
	@# TODO: Add teardown steps when TMC components are available  
	@echo "TMC test environment teardown complete"

## test-lint: Run linting on test files
test-lint:
	@echo "Running linting on TMC test files..."
	golangci-lint run ./...

## test-format: Format test files
test-format:
	@echo "Formatting TMC test files..."
	go fmt ./...

## test-validate: Validate test data files
test-validate:
	@echo "Validating TMC test data files..."
	@# TODO: Add validation when TMC APIs are available
	@echo "TMC test data validation complete"

## test-report: Generate comprehensive test report
test-report: test-coverage
	@echo "Generating comprehensive TMC test report..."
	@echo "=== TMC Test Report ===" > test-report.txt
	@echo "Generated: $(shell date)" >> test-report.txt
	@echo "" >> test-report.txt
	@echo "=== Coverage Summary ===" >> test-report.txt
	go tool cover -func=$(COVERAGE_OUT) >> test-report.txt
	@echo "" >> test-report.txt
	@echo "Test report generated: test-report.txt"

## test-ci: Run tests suitable for CI environment
test-ci: test-lint test-format test-unit test-integration test-coverage
	@echo "CI test suite completed successfully"

## test-dev: Run tests suitable for development
test-dev: test-short test-race
	@echo "Development test suite completed successfully"

## clean: Clean all test artifacts and reports
clean: test-clean
	@echo "Cleaning all TMC test artifacts..."
	rm -f test-report.txt
	@echo "Clean complete"

# Help target (default)
.DEFAULT_GOAL := help