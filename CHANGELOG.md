# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- **Serialization**: All operations now use exclusive locks instead of read locks. Operations on the same client connection are fully serialized to prevent write interleaving and simplify reconnection logic.

### Fixed

- **EOF Handling**: Treat `io.EOF` errors as transient and trigger automatic reconnection. This handles scenarios where network devices close idle connections (session timeouts, device restarts), improving reliability for long-running applications and idle connection scenarios.

## [0.1.0] - 2025-11-09

### Added

- Initial release

---

[Unreleased]: https://github.com/netascode/go-netconf/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/netascode/go-netconf/releases/tag/v0.1.0