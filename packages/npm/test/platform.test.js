"use strict";

const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");
const { archiveName, checksumFor, resolvePlatform, verifyChecksum } = require("../scripts/install");

test("resolves release archive names consistently with install.sh", () => {
  assert.equal(archiveName("v1.2.3", resolvePlatform("linux", "x64")), "gira_v1.2.3_linux_amd64.tar.gz");
  assert.equal(archiveName("v1.2.3", resolvePlatform("linux", "arm64")), "gira_v1.2.3_linux_arm64.tar.gz");
  assert.equal(archiveName("v1.2.3", resolvePlatform("darwin", "x64")), "gira_v1.2.3_darwin_amd64.tar.gz");
  assert.equal(archiveName("v1.2.3", resolvePlatform("darwin", "arm64")), "gira_v1.2.3_darwin_arm64.tar.gz");
  assert.equal(archiveName("v1.2.3", resolvePlatform("win32", "x64")), "gira_v1.2.3_windows_amd64.zip");
});

test("rejects unsupported platforms", () => {
  assert.throws(() => resolvePlatform("freebsd", "x64"), /unsupported OS/);
  assert.throws(() => resolvePlatform("linux", "ia32"), /unsupported architecture/);
  assert.throws(() => resolvePlatform("win32", "arm64"), /unsupported Windows architecture/);
});

test("reads checksum entry for archive", () => {
  const text = [
    "abc123  gira_v1.2.3_linux_amd64.tar.gz",
    "def456 *gira_v1.2.3_darwin_arm64.tar.gz",
  ].join("\n");
  assert.equal(checksumFor(text, "gira_v1.2.3_linux_amd64.tar.gz"), "abc123");
  assert.equal(checksumFor(text, "gira_v1.2.3_darwin_arm64.tar.gz"), "def456");
  assert.throws(() => checksumFor(text, "missing.tar.gz"), /does not include/);
});

test("checksum mismatch fails closed", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "gira-npm-test-"));
  try {
    const file = path.join(dir, "archive.tar.gz");
    fs.writeFileSync(file, "hello");
    assert.throws(() => verifyChecksum(file, "0000"), /checksum mismatch/);
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
});
