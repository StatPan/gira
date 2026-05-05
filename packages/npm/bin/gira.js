#!/usr/bin/env node
"use strict";

const fs = require("node:fs");
const path = require("node:path");
const { spawnSync } = require("node:child_process");

const bin = process.platform === "win32" ? "gira.exe" : "gira";
const binaryPath = path.join(__dirname, "..", "vendor", bin);

if (!fs.existsSync(binaryPath)) {
  console.error("gira binary is not installed. Reinstall @statpan/gira or run npm rebuild @statpan/gira.");
  process.exit(1);
}

const result = spawnSync(binaryPath, process.argv.slice(2), { stdio: "inherit" });
if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}
process.exit(result.status === null ? 1 : result.status);
