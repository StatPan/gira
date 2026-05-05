from __future__ import annotations

import hashlib
import importlib.metadata
import os
import platform
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
import zipfile
from pathlib import Path
from typing import BinaryIO, Dict, Optional

REPO = "StatPan/gira"


def package_version() -> str:
    return importlib.metadata.version("gira-cli")


def release_version() -> str:
    env_version = os.environ.get("GIRA_VERSION", "").strip()
    if env_version:
        return env_version
    version = package_version()
    if "dev" in version:
        return ""
    return version if version.startswith("v") else f"v{version}"


def resolve_platform(system: Optional[str] = None, machine: Optional[str] = None) -> Dict[str, str]:
    system = (system or platform.system()).lower()
    machine = (machine or platform.machine()).lower()
    os_map = {
        "linux": "linux",
        "darwin": "darwin",
        "windows": "windows",
    }
    arch_map = {
        "x86_64": "amd64",
        "amd64": "amd64",
        "arm64": "arm64",
        "aarch64": "arm64",
    }
    goos = os_map.get(system)
    goarch = arch_map.get(machine)
    if not goos:
        raise RuntimeError(f"unsupported OS: {system}")
    if not goarch:
        raise RuntimeError(f"unsupported architecture: {machine}")
    if goos == "windows" and goarch != "amd64":
        raise RuntimeError(f"unsupported Windows architecture: {machine}")
    return {
        "goos": goos,
        "goarch": goarch,
        "extension": "zip" if goos == "windows" else "tar.gz",
        "binary": "gira.exe" if goos == "windows" else "gira",
    }


def archive_name(version: str, info: Dict[str, str]) -> str:
    return f"gira_{version}_{info['goos']}_{info['goarch']}.{info['extension']}"


def base_url(version: str) -> str:
    return os.environ.get("GIRA_BASE_URL", "").strip() or f"https://github.com/{REPO}/releases/download/{version}"


def cache_dir(version: str, info: Dict[str, str]) -> Path:
    override = os.environ.get("GIRA_PYPI_CACHE_DIR", "").strip()
    root = Path(override).expanduser() if override else Path.home() / ".cache" / "gira-cli"
    return root / version / f"{info['goos']}_{info['goarch']}"


def binary_checksum_path(target: Path) -> Path:
    return target.with_name(f"{target.name}.sha256")


def file_checksum(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for chunk in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def cached_binary_valid(target: Path) -> bool:
    marker = binary_checksum_path(target)
    if not target.is_file() or not marker.is_file():
        return False
    if os.name != "nt" and not os.access(target, os.X_OK):
        return False
    return marker.read_text().strip() == file_checksum(target)


def read_url(url: str) -> bytes:
    if url.startswith("file://"):
        return Path(urllib.request.url2pathname(url[7:])).read_bytes()
    with urllib.request.urlopen(url) as response:
        return response.read()


def download_file(url: str, target: Path) -> None:
    target.write_bytes(read_url(url))


def checksum_for(checksums: str, archive: str) -> str:
    for line in checksums.splitlines():
        parts = line.strip().split()
        if len(parts) >= 2 and parts[1] in {archive, f"*{archive}"}:
            return parts[0]
    raise RuntimeError(f"checksum asset does not include {archive}")


def verify_checksum(path: Path, expected: str) -> None:
    actual = file_checksum(path)
    if actual != expected:
        raise RuntimeError(f"checksum mismatch for {path.name}")


def validate_archive_member(name: str) -> Path:
    if not name or "\x00" in name or "\\" in name or ":" in name:
        raise RuntimeError(f"unsafe archive member path: {name}")
    path = Path(name)
    if path.is_absolute() or ".." in path.parts:
        raise RuntimeError(f"unsafe archive member path: {name}")
    return path


def copy_member(source: BinaryIO, target: Path) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    with target.open("wb") as output:
        shutil.copyfileobj(source, output)


def extract_archive(path: Path, extension: str, output_dir: Path) -> None:
    if extension == "zip":
        with zipfile.ZipFile(path) as archive:
            for member in archive.infolist():
                target = output_dir / validate_archive_member(member.filename)
                if member.is_dir():
                    target.mkdir(parents=True, exist_ok=True)
                    continue
                with archive.open(member) as source:
                    copy_member(source, target)
        return
    with tarfile.open(path, "r:gz") as archive:
        for member in archive.getmembers():
            target = output_dir / validate_archive_member(member.name)
            if member.isdir():
                target.mkdir(parents=True, exist_ok=True)
                continue
            if not member.isfile():
                raise RuntimeError(f"unsupported archive member type: {member.name}")
            source = archive.extractfile(member)
            if source is None:
                raise RuntimeError(f"unable to extract archive member: {member.name}")
            with source:
                copy_member(source, target)


def install_cached_binary(source: Path, target: Path) -> None:
    target.parent.mkdir(parents=True, exist_ok=True)
    checksum = file_checksum(source)
    with tempfile.NamedTemporaryFile(prefix=f".{target.name}.", dir=target.parent, delete=False) as tmp:
        tmp_path = Path(tmp.name)
    marker_tmp_path: Optional[Path] = None
    try:
        shutil.copy2(source, tmp_path)
        tmp_path.chmod(tmp_path.stat().st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)
        os.replace(tmp_path, target)

        marker = binary_checksum_path(target)
        with tempfile.NamedTemporaryFile(
            mode="w", prefix=f".{marker.name}.", dir=target.parent, delete=False
        ) as marker_tmp:
            marker_tmp.write(f"{checksum}\n")
            marker_tmp_path = Path(marker_tmp.name)
        os.replace(marker_tmp_path, marker)
    finally:
        tmp_path.unlink(missing_ok=True)
        if marker_tmp_path is not None:
            marker_tmp_path.unlink(missing_ok=True)


def ensure_binary() -> Path:
    version = release_version()
    if not version:
        raise RuntimeError("gira-cli development package cannot resolve a release version; set GIRA_VERSION")
    info = resolve_platform()
    target_dir = cache_dir(version, info)
    target = target_dir / info["binary"]
    if cached_binary_valid(target):
        return target

    archive = archive_name(version, info)
    root = base_url(version)
    with tempfile.TemporaryDirectory(prefix="gira-pypi-") as tmp:
        tmpdir = Path(tmp)
        archive_path = tmpdir / archive
        checksums_path = tmpdir / "checksums.txt"
        download_file(f"{root}/{archive}", archive_path)
        download_file(f"{root}/checksums.txt", checksums_path)
        expected = checksum_for(checksums_path.read_text(), archive)
        verify_checksum(archive_path, expected)
        extract_archive(archive_path, info["extension"], tmpdir)
        extracted = tmpdir / f"gira_{version}_{info['goos']}_{info['goarch']}" / info["binary"]
        if not extracted.exists():
            raise RuntimeError(f"release archive did not contain {info['binary']}")
        install_cached_binary(extracted, target)
    return target


def main() -> int:
    try:
        binary = ensure_binary()
    except Exception as exc:
        print(f"gira pip wrapper: {exc}", file=sys.stderr)
        return 1
    completed = subprocess.run([str(binary), *sys.argv[1:]], check=False)
    return int(completed.returncode)


if __name__ == "__main__":
    raise SystemExit(main())
