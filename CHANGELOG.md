# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **Close Timeout Protection**: Added timeout protection to `Close()` and `reconnect()` methods to prevent indefinite blocking when using scrapligo v1.3.3. The library now uses `ConnectTimeout` (default 10s) to detect and handle the scrapligo bug where `driver.Close()` can block forever due to channel deadlock. This prevents application-wide deadlocks in the terraform-provider-iosxe and other users of the library.

## [0.5.0] - 2025-11-14

### Added

- **Lock-Denied Polling**: Implemented intelligent lock polling for `lock-denied` and `in-use` errors. Lock operations now use fixed 1-second polling intervals (instead of exponential backoff) and respect `LockReleaseTimeout` (default 120s) instead of `MaxRetries`. This provides much better lock acquisition behavior when datastores are temporarily locked by other sessions.

### Changed

- **Serialization**: All operations now use exclusive locks instead of read locks. Operations on the same client connection are fully serialized to prevent write interleaving and simplify reconnection logic.
- **Lock Timeout Behavior**: Lock-denied errors now poll for up to `LockReleaseTimeout` duration with 1-second intervals, ignoring `MaxRetries` configuration. Returns `ErrLockReleaseTimeout` when timeout is exceeded.
- **Transport Error Priority**: Transport errors (connection failures) are now handled before lock polling to ensure reconnection happens first when both error types occur simultaneously.

### Fixed

- **EOF Handling**: Treat `io.EOF` errors as transient and trigger automatic reconnection. This handles scenarios where network devices close idle connections (session timeouts, device restarts), improving reliability for long-running applications and idle connection scenarios.
- **Lock Polling Implementation**: Fixed `LockReleaseTimeout` option which was defined but not implemented. Lock-denied errors previously used standard exponential backoff and gave up after ~7-8 seconds (3 retries). Now correctly polls for configured timeout duration (default 120s).
- **Context Cancellation in Lock Polling**: Added defensive context cancellation check at the start of lock polling to respect context deadlines more precisely.

## [0.1.0] - 2025-11-09

### Added

- Initial release

---

[Unreleased]: https://github.com/netascode/go-netconf/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/netascode/go-netconf/compare/v0.1.0...v0.5.0
[0.1.0]: https://github.com/netascode/go-netconf/releases/tag/v0.1.0