"use strict";

function resolvePlatform(platform = process.platform, arch = process.arch) {
  const osMap = {
    linux: "linux",
    darwin: "darwin",
    win32: "windows",
  };
  const archMap = {
    x64: "amd64",
    arm64: "arm64",
  };

  const goos = osMap[platform];
  const goarch = archMap[arch];
  if (!goos) {
    throw new Error(`unsupported OS: ${platform}`);
  }
  if (!goarch) {
    throw new Error(`unsupported architecture: ${arch}`);
  }
  if (goos === "windows" && goarch !== "amd64") {
    throw new Error(`unsupported Windows architecture: ${arch}`);
  }

  const extension = goos === "windows" ? "zip" : "tar.gz";
  const binary = goos === "windows" ? "gira.exe" : "gira";
  return { goos, goarch, extension, binary };
}

function archiveName(version, platformInfo) {
  return `gira_${version}_${platformInfo.goos}_${platformInfo.goarch}.${platformInfo.extension}`;
}

module.exports = { resolvePlatform, archiveName };
