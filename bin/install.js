#!/usr/bin/env node

const crypto = require("crypto");
const fs = require("fs");
const os = require("os");
const path = require("path");
const https = require("https");

const MAX_DOWNLOAD_BYTES = 256 * 1024 * 1024;
const MAX_REDIRECTS = 5;

function getPlatform() {
  const platform = os.platform() === "win32" ? "windows" : os.platform();
  if (!["darwin", "linux", "windows"].includes(platform)) {
    throw new Error(`Unsupported platform: ${os.platform()}`);
  }
  const arch = os.arch() === "x64" ? "amd64" : os.arch() === "arm64" ? "arm64" : null;
  if (!arch) {
    throw new Error(`Unsupported architecture: ${os.arch()}`);
  }
  return `${platform}-${arch}`;
}

function getAssetName(platform) {
  return `db-mcp-server-${platform}${platform.startsWith("windows") ? ".exe" : ""}`;
}

function getReleaseURL(version, asset) {
  return `https://github.com/FreePeak/db-mcp-server/releases/download/v${version}/${asset}`;
}

function download(url, redirects = 0) {
  return new Promise((resolve, reject) => {
    if (redirects > MAX_REDIRECTS) {
      reject(new Error("too many redirects"));
      return;
    }
    const request = https.get(url, { headers: { "User-Agent": "db-mcp-server-installer" } }, (response) => {
      if ([301, 302, 303, 307, 308].includes(response.statusCode) && response.headers.location) {
        response.resume();
        download(new URL(response.headers.location, url).toString(), redirects + 1).then(resolve, reject);
        return;
      }
      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`download failed with HTTP ${response.statusCode}`));
        return;
      }
      const chunks = [];
      let size = 0;
      response.on("data", (chunk) => {
        size += chunk.length;
        if (size > MAX_DOWNLOAD_BYTES) {
          request.destroy(new Error("download exceeds 256 MiB"));
          return;
        }
        chunks.push(chunk);
      });
      response.on("end", () => resolve(Buffer.concat(chunks)));
      response.on("error", reject);
    });
    request.setTimeout(30_000, () => request.destroy(new Error("download timed out")));
    request.on("error", reject);
  });
}

function findChecksum(checksums, asset) {
  for (const line of checksums.toString("utf8").split(/\r?\n/)) {
    const fields = line.trim().split(/\s+/);
    if (fields.length < 2) continue;
    const name = fields[fields.length - 1].replace(/^\*/, "");
    if (name === asset) return fields[0].toLowerCase();
  }
  throw new Error(`no SHA-256 checksum found for ${asset}`);
}

async function install() {
  const packageJson = require("../package.json");
  const platform = getPlatform();
  const asset = getAssetName(platform);
  const version = packageJson.version;
  const destination = path.join(__dirname, platform.startsWith("windows") ? "db-mcp-server.exe" : "db-mcp-server");
  const temporary = `${destination}.${process.pid}.tmp`;

  console.log(`Installing db-mcp-server v${version} for ${platform}...`);
  const checksums = await download(getReleaseURL(version, "SHA256SUMS"));
  const expected = findChecksum(checksums, asset);
  const binary = await download(getReleaseURL(version, asset));
  const actual = crypto.createHash("sha256").update(binary).digest("hex");
  if (actual !== expected) {
    throw new Error(`SHA-256 mismatch for ${asset}`);
  }

  fs.writeFileSync(temporary, binary, { mode: 0o755 });
  fs.rmSync(destination, { force: true });
  fs.renameSync(temporary, destination);
  if (platform.startsWith("windows")) {
    fs.chmodSync(destination, 0o755);
  }
  console.log(`Installed db-mcp-server to ${destination}`);
}

install().catch((error) => {
  console.error(`Failed to install db-mcp-server: ${error.message}`);
  process.exit(1);
});
