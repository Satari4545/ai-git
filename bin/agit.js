#!/usr/bin/env node

const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const path = require("node:path");

const executable = process.platform === "win32" ? "agit.exe" : "agit";
const binaryPath = path.join(__dirname, "..", "vendor", executable);

if (!fs.existsSync(binaryPath)) {
  console.error("agit binary is missing. Try reinstalling the package.");
  process.exit(1);
}

const result = spawnSync(binaryPath, process.argv.slice(2), {
  stdio: "inherit"
});

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}

process.exit(result.status ?? 0);
