# Changelog

## [Unreleased]

### Added
- `max_rows` per-database configuration option (all engines): truncates query results with an explicit `[Truncated]` notice to protect agent context windows from large result sets

### Fixed
- **Read-only bypass (security)**: write statements (`INSERT`/`UPDATE`/`DELETE`/DDL/data-modifying CTEs/stacked statements) executed through the `query_*` tool no longer bypass the per-database `read_only: true` guard; statement classification strips comments and string literals and defaults to deny for unrecognized leading keywords
- **Transactions were stubbed**: the `transaction_*` tools' `begin` action silently committed immediately, while `execute`, `commit`, and `rollback` returned success without doing anything. All four actions now operate on a real stored transaction keyed by the returned `transactionId`; unknown IDs fail with a clear error instead of faking success

## [v1.6.1] - 2025-04-01

### Added
- OpenAI Agents SDK compatibility by adding Items property to array parameters
- Test script for verifying OpenAI Agents SDK compatibility

### Fixed
- Issue #8: Array parameters in tool definitions now include required `items` property
- JSON Schema validation errors in OpenAI Agents SDK integration

## [v1.6.0] - 2023-03-31

### Changed
- Upgraded cortex dependency from v1.0.3 to v1.0.4

## [] - 2023-03-31

### Added
- Internal logging system for improved debugging and monitoring
- Logger implementation for all packages

### Fixed
- Connection issues with PostgreSQL databases
- Restored functionality for all MCP tools
- Eliminated non-JSON RPC logging in stdio mode

## [] - 2023-03-25

### Added
- Initial release of DB MCP Server
- Multi-database connection support
- Tool generation for database operations
- README with guidelines on using tools in Cursor

