@echo off
setlocal
"%~dp0db-mcp-server.exe" %*
exit /b %ERRORLEVEL%
