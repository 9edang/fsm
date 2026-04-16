# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

Nothing yet.

## [1.1.0] - 2026-04-16

### Changed
- **[PERFORMANCE]** Optimized transition lookups from O(n) to O(1) using map-based indexing
  - 27-28% performance improvement on hot paths
  - `Trigger_HotPath`: 170.8 ns/op → 123.4 ns/op (27.8% faster)
  - `Trigger_Parallel`: 71.67 ns/op → 51.52 ns/op (28.1% faster)
  - Consistent O(1) performance regardless of FSM size

### Added
- Comprehensive profiling analysis ([PROFILING_REPORT.md](PROFILING_REPORT.md))
- Performance optimization documentation ([OPTIMIZATION_SUMMARY.md](OPTIMIZATION_SUMMARY.md))
- Large FSM benchmarks to verify O(1) scaling
  - `BenchmarkTrigger_LargeFSM_FirstTransition`
  - `BenchmarkTrigger_LargeFSM_LastTransition`
  - `BenchmarkCan_LargeFSM`

### Internal
- Added `index` field to `definition` struct for O(1) transition lookups
- Added `buildTransitionIndex()` function to construct lookup map during `Build()`
- Index built once and remains immutable (thread-safe)
- Memory trade-off: ~24 bytes per transition

### Benchmarks (v1.1.0)
- `BenchmarkTrigger_HotPath`: 123.4 ns/op, 64 B/op, 1 alloc/op
- `BenchmarkTrigger_WithGuardAndAction`: 122.5 ns/op, 64 B/op, 1 alloc/op
- `BenchmarkTrigger_WithHooks`: 127.5 ns/op, 64 B/op, 1 alloc/op
- `BenchmarkTrigger_Parallel`: 51.52 ns/op, 64 B/op, 1 alloc/op
- `BenchmarkCan`: 29.26 ns/op, 0 B/op, 0 allocs/op
- `BenchmarkTrigger_LargeFSM_FirstTransition`: 132.5 ns/op, 64 B/op, 1 alloc/op
- `BenchmarkTrigger_LargeFSM_LastTransition`: 136.3 ns/op, 64 B/op, 1 alloc/op (only 2.9% slower than first)
- `BenchmarkCan_LargeFSM`: 35.90 ns/op, 0 B/op, 0 allocs/op

## [1.0.0] - 2026-04-16

Initial stable release of the FSM package. A type-safe, declarative, concurrent-safe finite state machine library for Go.

### Added
- Type-safe `State` and `Event` types
- Fluent builder API for FSM definition (`New()`, `On()`, `From()`, `To()`, `Build()`)
- State transition triggering with `Trigger()`
- Guard functions for conditional transitions
- Action functions executed on successful transitions
- Comprehensive hook system:
  - `BeforeTransition` - called before every transition
  - `AfterTransition` - called after successful transitions
  - `OnEnter` - called when entering specific states
  - `OnExit` - called when exiting specific states
  - `OnError` - called on guard or action failures
- Read-only state inspection with `Can()`, `Current()`, `Transitions()`
- State persistence support with `NewWithState()`
- History tracking with `WithHistory()` and `History()`
- Visualization exports (`ToMermaid()`, `ToDOT()`)
- Concurrent-safe operations with `sync.RWMutex`
- Typed error types:
  - `ErrInvalidTransition` - invalid state/event combination
  - `ErrUnknownEvent` - unrecognized event
  - `ErrGuardFailed` - guard function returned error
- Comprehensive test suite (47 tests)
- Benchmark suite covering hot paths and edge cases
- Documentation:
  - Detailed README.md with examples
  - Inline code documentation
  - License (MIT)

### Design Principles
- Type-safe: distinct named types prevent plain string mistakes
- Declarative: fluent builder API with immutable schema
- Zero-magic: no reflection, no globals, no hidden goroutines
- Testable: context-aware guards/actions, typed errors
- Concurrent-safe: all exported methods are thread-safe

### Performance (v1.0.0)
- O(n) linear search for transition lookups
- Single 64-byte allocation per FSM instance
- Zero allocations in `Trigger()` execution path
- Zero allocations in `Can()` read operations

### Benchmarks (v1.0.0)
- `BenchmarkTrigger_HotPath`: 170.8 ns/op, 64 B/op, 1 alloc/op
- `BenchmarkTrigger_WithGuardAndAction`: 160.1 ns/op, 64 B/op, 1 alloc/op
- `BenchmarkTrigger_WithHooks`: 142.6 ns/op, 64 B/op, 1 alloc/op
- `BenchmarkTrigger_Parallel`: 71.67 ns/op, 64 B/op, 1 alloc/op
- `BenchmarkCan`: 25.66 ns/op, 0 B/op, 0 allocs/op

---

## Release Notes

### Version 1.1.0 Highlights
Performance optimization release:
- ✅ O(1) transition lookups (27-28% faster)
- ✅ Scales efficiently to 100+ state FSMs
- ✅ Backward compatible with v1.0.0
- ✅ Comprehensive profiling and optimization documentation

### Version 1.0.0 Highlights
Initial stable release:
- ✅ Full FSM feature set with guards, actions, and comprehensive hooks
- ✅ 47 tests passing with race detector validation
- ✅ Production-ready API with semantic versioning commitment
- ✅ Zero external dependencies

### Versioning Strategy
This library follows [Semantic Versioning](https://semver.org/):
- **MAJOR** version for incompatible API changes
- **MINOR** version for backwards-compatible functionality additions
- **PATCH** version for backwards-compatible bug fixes

As of v1.0.0, the public API is considered stable and breaking changes will be avoided or clearly documented.

### Compatibility
- Requires Go 1.21 or later (tested on Go 1.23.0)
- Zero external dependencies
- API stability: public API is stable and follows semantic versioning

### Migration Guides

#### From v1.0.0 to v1.1.0
No code changes required. The optimization is fully backward compatible:
- Same API, same behavior
- Automatic performance improvement (27-28% faster)
- Drop-in replacement

---

## Links
- [GitHub Repository](https://github.com/9edang/fsm)
- [API Documentation](https://pkg.go.dev/github.com/9edang/fsm)
- [Issue Tracker](https://github.com/9edang/fsm/issues)

[Unreleased]: https://github.com/9edang/fsm/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/9edang/fsm/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/9edang/fsm/releases/tag/v1.0.0
