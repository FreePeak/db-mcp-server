#!/usr/bin/env node

const path = require("path");
const { spawnSync } = require("child_process");

function getBinaryPath() {
  return path.join(__dirname, process.platform === "win32" ? "db-mcp-server.exe" : "db-mcp-server");
}

const result = spawnSync(getBinaryPath(), process.argv.slice(2), {
  stdio: "inherit",
  env: { ...process.env },
});

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}
process.exit(result.status ?? 1);
