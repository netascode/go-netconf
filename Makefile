.PHONY: help test lint security coverage benchmark clean fmt verify tools ci

# Default target
help:
	@echo "Available targets:"
	@echo "  test       - Run all tests"
	@echo "  lint       - Run linters (golangci-lint)"
	@echo "  security   - Run security scanner (gosec)"
	@echo "  coverage   - Run tests with coverage report"
	@echo "  benchmark  - Run benchmarks"
	@echo "  clean      - Clean build artifacts"
	@echo "  fmt        - Format code"
	@echo "  verify     - Run all checks (test, lint, security)"
	@echo "  tools      - Install development tools"
	@echo "  ci         - Run CI pipeline checks"

# Run tests
test:
	@echo "Running tests..."
	go test -v -race ./...

# Run linters
lint:
	@echo "Running linters..."
	golangci-lint run

# Run security scanner
security:
	@echo "Running security scanner..."
	@if which gosec > /dev/null 2>&1; then \
		gosec -quiet ./...; \
	else \
		echo "WARNING: gosec not installed. Run 'make tools' to install it."; \
		echo "Skipping security scan..."; \
	fi

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f coverage.out coverage.html
	go clean -testcache

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w .

# Run all checks
verify: fmt test lint security
	@echo "All checks passed!"

# Install development tools
tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest

# CI pipeline checks (used in GitHub Actions)
ci: test lint security
	@echo "CI checks passed!"
