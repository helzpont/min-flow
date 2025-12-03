# Min-Flow Makefile
# Build, test, and development tasks

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=$(GOCMD) fmt
GOMOD=$(GOCMD) mod

# Directories
BIN_DIR=bin
EXAMPLES_DIR=examples

# Build flags
LDFLAGS=-ldflags="-s -w"

# Default target
.PHONY: all
all: build

# Build all examples
.PHONY: build
build: $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/basic ./$(EXAMPLES_DIR)/basic
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/combine ./$(EXAMPLES_DIR)/combine
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/errors ./$(EXAMPLES_DIR)/errors
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/interceptors ./$(EXAMPLES_DIR)/interceptors
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/parallel ./$(EXAMPLES_DIR)/parallel
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/resilience ./$(EXAMPLES_DIR)/resilience

# Create bin directory
$(BIN_DIR):
	mkdir -p $(BIN_DIR)

# Run all tests
.PHONY: test
test:
	$(GOTEST) -v -race ./...

# Run tests with coverage
.PHONY: coverage
coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Run benchmarks
.PHONY: bench
bench:
	cd benchmarks && $(GOTEST) -bench=. -benchmem -run=^$

# Lint and vet
.PHONY: lint
lint:
	$(GOVET) ./...
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

# Format code
.PHONY: fmt
fmt:
	$(GOFMT) ./...

# Tidy dependencies
.PHONY: tidy
tidy:
	$(GOMOD) tidy

# Clean build artifacts
.PHONY: clean
clean:
	find $(BIN_DIR) -type f ! -name '.gitkeep' -delete
	rm -f coverage.out coverage.html
	rm -f benchmarks/*.test benchmarks/*.prof

# Run a specific example (usage: make run EXAMPLE=basic)
.PHONY: run
run: build
	@if [ -z "$(EXAMPLE)" ]; then \
		echo "Usage: make run EXAMPLE=<name>"; \
		echo "Available examples: basic, combine, errors, interceptors, parallel, resilience"; \
		exit 1; \
	fi
	./$(BIN_DIR)/$(EXAMPLE)

# Development workflow: format, lint, test
.PHONY: check
check: fmt lint test

# Help
.PHONY: help
help:
	@echo "Min-Flow Makefile"
	@echo ""
	@echo "Usage:"
	@echo "  make              Build all examples"
	@echo "  make build        Build all examples to bin/"
	@echo "  make test         Run all tests with race detector"
	@echo "  make coverage     Run tests with coverage report"
	@echo "  make bench        Run benchmarks"
	@echo "  make lint         Run go vet and golangci-lint"
	@echo "  make fmt          Format all Go files"
	@echo "  make tidy         Tidy go.mod dependencies"
	@echo "  make clean        Remove build artifacts"
	@echo "  make run EXAMPLE=<name>  Run a specific example"
	@echo "  make check        Format, lint, and test (CI workflow)"
	@echo "  make help         Show this help"
