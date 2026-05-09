import { existsSync, readdirSync, readFileSync } from "node:fs";
import { dirname, join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const problems = [];

const reactServerDomPackages = [
  "react-server-dom-webpack",
  "react-server-dom-parcel",
  "react-server-dom-turbopack",
];

const packageRules = {
  next: validateNext,
  react: validateReact19,
  "react-dom": validateReact19,
  ...Object.fromEntries(reactServerDomPackages.map((name) => [name, validateReactServerDom])),
};

for (const file of findPackageJsonFiles(root)) {
  const manifest = readJson(file);
  for (const field of ["dependencies", "devDependencies", "optionalDependencies", "peerDependencies"]) {
    for (const [name, range] of Object.entries(manifest[field] ?? {})) {
      checkDependency(`${relative(root, file)} ${field}.${name}`, name, String(range));
    }
  }
}

checkPackageLock(join(root, "package-lock.json"));
checkPnpmLock(join(root, "pnpm-lock.yaml"));

if (problems.length > 0) {
  console.error("Security dependency check failed:");
  for (const problem of problems) console.error(`- ${problem}`);
  process.exit(1);
}

console.log("Security dependency check passed.");

function checkDependency(location, name, rawVersion) {
  const rule = packageRules[name];
  if (!rule) return;
  const range = parseVersionRange(rawVersion);
  if (!range) {
    problems.push(`${location}: cannot evaluate version range "${rawVersion}"`);
    return;
  }
  const message = rule(range);
  if (message) problems.push(`${location}: ${rawVersion} ${message}`);
}

function validateReact19(range) {
  return rangeIntersectsAny(range, [
    interval("19.0.0", "19.2.6"),
  ])
    ? "can resolve below the local React 19 floor 19.2.6"
    : null;
}

function validateNext(range) {
  return rangeIntersectsAny(range, [
    interval("13.0.0", "15.5.16"),
    interval("16.0.0", "16.2.5"),
  ])
    ? "can resolve to Next.js affected by GHSA-8h8q-6873-q5fj; use >=15.5.16 or >=16.2.5"
    : null;
}

function validateReactServerDom(range) {
  return rangeIntersectsAny(range, [
    interval("19.0.0", "19.0.6"),
    interval("19.1.0", "19.1.7"),
    interval("19.2.0", "19.2.6"),
  ])
    ? "can resolve to a React Server DOM version affected by GHSA-rv78-f8rc-xrxh"
    : null;
}

function parseVersionRange(raw) {
  const text = raw.trim();
  if (
    text === "" ||
    text === "*" ||
    /(?:^|[./])(?:x|X|\*)/.test(text) ||
    /^(?:workspace:|file:|link:|git\+|https?:)/.test(text) ||
    text.includes(" - ")
  ) {
    return null;
  }
  const alternatives = text.split("||").map((part) => parseRangeAlternative(part.trim()));
  if (alternatives.some((part) => part == null)) return null;
  return alternatives;
}

function parseRangeAlternative(text) {
  text = text.replace(/(>=|>|<=|<|=)\s+/g, "$1");
  if (text === "") return null;
  const bare = text.match(/^[=v]*([0-9]+\.[0-9]+\.[0-9]+)$/);
  if (bare) {
    const version = parseVersion(bare[1]);
    return { lower: version, lowerInclusive: true, upper: incrementPatch(version), upperInclusive: false };
  }
  const caret = text.match(/^\^([0-9]+\.[0-9]+\.[0-9]+)$/);
  if (caret) {
    const version = parseVersion(caret[1]);
    return { lower: version, lowerInclusive: true, upper: incrementCaret(version), upperInclusive: false };
  }
  const tilde = text.match(/^~([0-9]+\.[0-9]+\.[0-9]+)$/);
  if (tilde) {
    const version = parseVersion(tilde[1]);
    return { lower: version, lowerInclusive: true, upper: { major: version.major, minor: version.minor + 1, patch: 0 }, upperInclusive: false };
  }

  const parts = text.split(/\s+/);
  const range = { lower: null, lowerInclusive: true, upper: null, upperInclusive: false };
  for (const part of parts) {
    const match = part.match(/^(>=|>|<=|<|=)?v?([0-9]+\.[0-9]+\.[0-9]+)$/);
    if (!match) return null;
    const op = match[1] ?? "=";
    const version = parseVersion(match[2]);
    switch (op) {
      case ">=":
        range.lower = maxBound(range.lower, version);
        range.lowerInclusive = true;
        break;
      case ">":
        range.lower = maxBound(range.lower, incrementPatch(version));
        range.lowerInclusive = false;
        break;
      case "<=":
        range.upper = minBound(range.upper, incrementPatch(version));
        range.upperInclusive = true;
        break;
      case "<":
        range.upper = minBound(range.upper, version);
        range.upperInclusive = false;
        break;
      case "=":
        range.lower = maxBound(range.lower, version);
        range.lowerInclusive = true;
        range.upper = minBound(range.upper, incrementPatch(version));
        range.upperInclusive = false;
        break;
      default:
        return null;
    }
  }
  return range.lower || range.upper ? range : null;
}

function parseVersion(value) {
  const [major, minor, patch] = value.split(".").map(Number);
  return { major, minor, patch };
}

function interval(lower, upper) {
  return { lower: parseVersion(lower), lowerInclusive: true, upper: parseVersion(upper), upperInclusive: false };
}

function rangeIntersectsAny(range, vulnerableIntervals) {
  return range.some((candidate) => vulnerableIntervals.some((vulnerable) => rangesIntersect(candidate, vulnerable)));
}

function rangesIntersect(left, right) {
  const lower = maxBound(left.lower, right.lower);
  const upper = minBound(left.upper, right.upper);
  if (!lower || !upper) return true;
  return compareVersion(lower, upper) < 0;
}

function maxBound(left, right) {
  if (!left) return right;
  if (!right) return left;
  return compareVersion(left, right) >= 0 ? left : right;
}

function minBound(left, right) {
  if (!left) return right;
  if (!right) return left;
  return compareVersion(left, right) <= 0 ? left : right;
}

function incrementPatch(version) {
  return { major: version.major, minor: version.minor, patch: version.patch + 1 };
}

function incrementCaret(version) {
  if (version.major > 0) return { major: version.major + 1, minor: 0, patch: 0 };
  if (version.minor > 0) return { major: 0, minor: version.minor + 1, patch: 0 };
  return incrementPatch(version);
}

function compareVersion(left, right) {
  for (const key of ["major", "minor", "patch"]) {
    if (left[key] !== right[key]) return left[key] - right[key];
  }
  return 0;
}

function checkPackageLock(file) {
  if (!existsSync(file)) return;
  const lock = readJson(file);
  for (const [path, entry] of Object.entries(lock.packages ?? {})) {
    const name = path.split("node_modules/").pop();
    if (!name || !entry?.version) continue;
    checkDependency(`${relative(root, file)} ${path}`, name, entry.version);
  }
}

function checkPnpmLock(file) {
  if (!existsSync(file)) return;
  const text = readFileSync(file, "utf8");
  const names = ["next", "react", "react-dom", ...reactServerDomPackages];
  for (const name of names) {
    const escaped = name.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const pattern = new RegExp(`^\\s{2}${escaped}@([^:\\s]+):`, "gm");
    let match;
    while ((match = pattern.exec(text))) {
      checkDependency(`${relative(root, file)} ${name}@${match[1]}`, name, match[1]);
    }
  }
}

function findPackageJsonFiles(dir) {
  const files = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    if (entry.name === "node_modules" || entry.name === ".git" || entry.name === "dist") continue;
    const path = join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...findPackageJsonFiles(path));
    } else if (entry.name === "package.json") {
      files.push(path);
    }
  }
  return files;
}

function readJson(file) {
  return JSON.parse(readFileSync(file, "utf8"));
}
