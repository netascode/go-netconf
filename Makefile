.PHONY: help test lint security coverage benchmark clean fmt verify tools ci license check-license

# Default target
help:
	@echo "Available targets:"
	@echo "  test          - Run all tests"
	@echo "  lint          - Run linters (golangci-lint with gosec)"
	@echo "  security      - Run vulnerability check (govulncheck)"
	@echo "  coverage      - Run tests with coverage report"
	@echo "  benchmark     - Run benchmarks"
	@echo "  clean         - Clean build artifacts"
	@echo "  fmt           - Format code"
	@echo "  license       - Add license headers to all Go files"
	@echo "  check-license - Verify license headers are present"
	@echo "  verify        - Run all checks (test, lint, security, license)"
	@echo "  tools         - Install development tools"
	@echo "  ci            - Run CI pipeline checks (test, lint, security, license)"

# Run tests
test:
	@echo "Running tests..."
	go test -v -race ./...

# Run linters
lint:
	@echo "Running linters..."
	golangci-lint run

# Run security scanner (vulnerability check)
security:
	@echo "Running vulnerability check..."
	@if which govulncheck > /dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "WARNING: govulncheck not installed. Run 'make tools' to install it."; \
		echo "Skipping vulnerability check..."; \
	fi

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	@# Exclude examples from coverage
	go list ./... | grep -v /examples | xargs go test -race -coverprofile=coverage.out -covermode=atomic
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
verify: fmt test lint security check-license
	@echo "All checks passed!"

# Install development tools
tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2
	go install golang.org/x/vuln/cmd/govulncheck@v1.1.3
	go install github.com/google/addlicense@v1.1.1

# Add license headers to all Go files
# Uses Google's addlicense tool (github.com/google/addlicense)
# Adds MPL-2.0 + SPDX headers with your name as copyright holder
# Existing headers are preserved (no re-processing)
# Note: Requires addlicense in PATH or ~/go/bin - install with 'make tools'
license:
	@echo "Adding license headers to Go files..."
	@if [ -z "$(HOME)" ] || [ ! -d "$(HOME)" ]; then \
		echo "Error: Invalid HOME directory (required for ~/go/bin tool installation)"; \
		echo "       If running in Docker/container, ensure HOME is set and ~/go/bin exists"; exit 1; \
	fi
	@PATH="$(HOME)/go/bin:$$PATH" && \
	command -v addlicense >/dev/null 2>&1 || { echo "Error: addlicense not found. Install with: make tools"; exit 1; } && \
	find . -name "*.go" -not -path "./vendor/*" -not -path "./examples/*" -print0 | \
		xargs -0 addlicense -c "Daniel Schmidt" -l mpl -s=only -y 2025 -v
	@echo "License header addition complete!"

# Check that all Go files have license headers
# Uses addlicense in check mode - accepts ANY copyright holder name
# Only verifies that MPL-2.0 + SPDX headers exist
# Note: Requires addlicense in PATH or ~/go/bin - install with 'make tools'
check-license:
	@echo "Checking license headers..."
	@if [ -z "$(HOME)" ] || [ ! -d "$(HOME)" ]; then \
		echo "Error: Invalid HOME directory (required for ~/go/bin tool installation)"; \
		echo "       If running in Docker/container, ensure HOME is set and ~/go/bin exists"; exit 1; \
	fi
	@PATH="$(HOME)/go/bin:$$PATH" && \
	command -v addlicense >/dev/null 2>&1 || { echo "Error: addlicense not found. Install with: make tools"; exit 1; } && \
	find . -name "*.go" -not -path "./vendor/*" -not -path "./examples/*" -print0 | \
		if xargs -0 addlicense -check -l mpl -s=only -y 2025; then \
			echo "✓ All Go files have license headers!"; \
		else \
			echo "✗ Some files are missing license headers. Run 'make license' to add them."; exit 1; \
		fi

# CI pipeline checks (used in GitHub Actions)
ci: test lint security check-license
	@echo "CI checks passed!"
