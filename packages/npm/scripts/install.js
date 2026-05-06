"use strict";

const crypto = require("node:crypto");
const fs = require("node:fs");
const https = require("node:https");
const os = require("node:os");
const path = require("node:path");
const { spawnSync } = require("node:child_process");
const { fileURLToPath } = require("node:url");
const { archiveName, resolvePlatform } = require("./platform");

const repo = "StatPan/gira";
const packageRoot = path.join(__dirname, "..");

function packageVersion() {
  return JSON.parse(fs.readFileSync(path.join(packageRoot, "package.json"), "utf8")).version;
}

function releaseVersion() {
  if (process.env.GIRA_VERSION) {
    return process.env.GIRA_VERSION;
  }
  const version = packageVersion();
  if (version.includes("dev")) {
    return "";
  }
  return version.startsWith("v") ? version : `v${version}`;
}

function releaseBaseURL(version) {
  return process.env.GIRA_BASE_URL || `https://github.com/${repo}/releases/download/${version}`;
}

function download(url, target) {
  if (url.startsWith("file://")) {
    fs.copyFileSync(fileURLToPath(url), target);
    return Promise.resolve();
  }
  return new Promise((resolve, reject) => {
    https.get(url, (response) => {
      if (response.statusCode >= 300 && response.statusCode < 400 && response.headers.location) {
        response.resume();
        download(response.headers.location, target).then(resolve, reject);
        return;
      }
      if (response.statusCode !== 200) {
        response.resume();
        reject(new Error(`download failed ${response.statusCode}: ${url}`));
        return;
      }
      const file = fs.createWriteStream(target);
      response.pipe(file);
      file.on("finish", () => file.close(resolve));
      file.on("error", reject);
    }).on("error", reject);
  });
}

function checksumFor(checksumsText, archive) {
  for (const line of checksumsText.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed) {
      continue;
    }
    const parts = trimmed.split(/\s+/);
    if (parts.length >= 2 && (parts[1] === archive || parts[1] === `*${archive}`)) {
      return parts[0];
    }
  }
  throw new Error(`checksum asset does not include ${archive}`);
}

function verifyChecksum(filePath, expected) {
  const actual = crypto.createHash("sha256").update(fs.readFileSync(filePath)).digest("hex");
  if (actual !== expected) {
    throw new Error(`checksum mismatch for ${path.basename(filePath)}`);
  }
}

function extractArchive(archivePath, extension, outputDir) {
  let result;
  if (extension === "zip") {
    result = spawnSync("powershell", ["-NoProfile", "-Command", "Expand-Archive", "-LiteralPath", archivePath, "-DestinationPath", outputDir, "-Force"], { stdio: "inherit" });
  } else {
    result = spawnSync("tar", ["-xzf", archivePath, "-C", outputDir], { stdio: "inherit" });
  }
  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    throw new Error(`extract failed with status ${result.status}`);
  }
}

async function install() {
  const version = releaseVersion();
  if (!version) {
    console.log("gira npm wrapper: development package version; skipping binary download");
    return;
  }

  const platform = resolvePlatform();
  const archive = archiveName(version, platform);
  const base = releaseBaseURL(version);
  const tempDir = fs.mkdtempSync(path.join(os.tmpdir(), "gira-npm-"));
  const vendorDir = path.join(packageRoot, "vendor");
  const archivePath = path.join(tempDir, archive);
  const checksumsPath = path.join(tempDir, "checksums.txt");

  try {
    await download(`${base}/${archive}`, archivePath);
    await download(`${base}/checksums.txt`, checksumsPath);
    const expected = checksumFor(fs.readFileSync(checksumsPath, "utf8"), archive);
    verifyChecksum(archivePath, expected);
    extractArchive(archivePath, platform.extension, tempDir);

    const extracted = path.join(tempDir, `gira_${version}_${platform.goos}_${platform.goarch}`, platform.binary);
    if (!fs.existsSync(extracted)) {
      throw new Error(`release archive did not contain ${platform.binary}`);
    }
    fs.mkdirSync(vendorDir, { recursive: true });
    const target = path.join(vendorDir, platform.binary);
    fs.copyFileSync(extracted, target);
    fs.chmodSync(target, 0o755);
    console.log(`installed gira ${version} to ${target}`);
  } finally {
    fs.rmSync(tempDir, { recursive: true, force: true });
  }
}

if (require.main === module) {
  install().catch((error) => {
    console.error(`gira npm wrapper: ${error.message}`);
    process.exit(1);
  });
}

module.exports = {
  install,
  checksumFor,
  verifyChecksum,
  releaseVersion,
  archiveName,
  resolvePlatform,
};
