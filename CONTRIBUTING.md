# Contributing to go-netconf

Thank you for your interest in contributing to go-netconf! This document provides guidelines for contributing to the project.

## Code of Conduct

This project adheres to a [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check existing issues to avoid duplicates. Include:

- Clear title and description with steps to reproduce
- Expected vs actual behavior with code samples
- Go version, OS information, device type/version
- NETCONF capabilities supported by the device

### Suggesting Features

Feature suggestions are welcome! Please:

- Check existing feature requests to avoid duplicates
- Provide clear use cases and explain why it's useful
- Consider RFC compliance (RFC 6241, 6242) and device compatibility
- Consider implementation complexity and security implications

### Pull Requests

1. Fork the repository and create a branch from `main`
2. Make your changes following the coding guidelines below
3. Add tests for any new functionality
4. Add SPDX headers to new Go files - see Headers section (contributors: add manually with YOUR copyright, do NOT run `make license`)
5. Ensure all checks pass: `make test`, `make lint`, `make security`
6. Update documentation as needed
7. Write clear commit messages following conventional commits
8. Submit a pull request

## Development Setup

```bash
# Clone your fork
git clone https://github.com/YOUR-USERNAME/go-netconf.git
cd go-netconf

# Install dependencies
go mod download

# Run all checks
make verify
```

**Prerequisites**: Go 1.24+, golangci-lint, Make (optional)

## Coding Guidelines

### Go Code Style

- Follow [Effective Go](https://go.dev/doc/effective_go) and [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `go fmt` for formatting
- Write clear comments for exported functions
- Follow NETCONF terminology from RFC 6241

### File Headers

All Go source files must include SPDX license identifier and copyright notice:

```go
// SPDX-License-Identifier: MPL-2.0
// Copyright (c) 2025 Daniel Schmidt

package netconf
```

#### Adding Headers

The project uses [Google's addlicense](https://github.com/google/addlicense) tool for license header management.

**Install addlicense:**
```bash
go install github.com/google/addlicense@v1.1.1
# Or use: make tools
```

**Version Note**: Use the exact version shown above (`@v1.1.1`) to match the CI environment. Using `@latest` may cause version mismatches between local development and CI checks.

**Add headers to new files:**
```bash
make license
```

**Verify headers:**
```bash
make check-license
```

**Important Notes:**
- **For Maintainers**: Use `make license` to add headers with project default copyright holder
- **For Contributors**: Do NOT use `make license` - it will overwrite your copyright holder
- **For Contributors**: Manually add headers to new files with YOUR organization's copyright:
  ```go
  // SPDX-License-Identifier: MPL-2.0
  // Copyright (c) 2025 Your Organization Name

  package netconf
  ```
- Then run `make check-license` to verify format compliance (accepts any copyright holder)
- Both SPDX-first and Copyright-first formats are accepted by the check command

### Testing

- Write unit tests for all new functionality
- Use table-driven tests with edge cases and error conditions
- Test with mock NETCONF sessions for unit tests
- Add integration tests for device compatibility
- Run race detector: `go test -race`

### Security Testing

Test all code for security vulnerabilities:

- **Lock management**: Ensure defer unlock is always used
- **Input validation**: Test malformed XML, oversized payloads, invalid characters
- **Error handling**: Verify retry logic doesn't leak credentials or sensitive data
- **SSH security**: Verify host key checking enforced by default
- **Concurrent access**: Test thread safety with race detector

### Documentation

- Document all exported functions, types, and constants
- Include usage examples in godoc comments
- Update README.md for significant changes
- Document NETCONF capabilities required and security implications

## NETCONF RFC Compliance

This project aims for full RFC compliance:

- **RFC 6241**: NETCONF Protocol - All base operations must comply
- **RFC 6242**: NETCONF over SSH - Transport layer compliance required

When implementing NETCONF operations, reference relevant RFC sections in code comments.

## Review Process

All PRs must meet these requirements:

1. At least one approval
2. CI passes (tests, lint, coverage, security)
3. Code follows style guidelines
4. SPDX headers present in all Go files
5. Documentation updated
6. Security implications considered
7. Breaking changes discussed (requires major version bump)

## Questions?

- Open an issue for questions
- Start a discussion in GitHub Discussions
- Reach out to maintainers

## License

By contributing, you agree that your contributions will be licensed under the Mozilla Public License Version 2.0.
