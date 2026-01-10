# Changelog

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

