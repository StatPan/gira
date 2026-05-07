import hashlib
import os
import tarfile
import tempfile
import unittest
import zipfile
from pathlib import Path
from unittest import mock

from gira_cli import installer


class InstallerTest(unittest.TestCase):
    def test_archive_names_match_release_assets(self):
        cases = [
            ("Linux", "x86_64", "gira_v1.2.3_linux_amd64.tar.gz"),
            ("Linux", "aarch64", "gira_v1.2.3_linux_arm64.tar.gz"),
            ("Darwin", "x86_64", "gira_v1.2.3_darwin_amd64.tar.gz"),
            ("Darwin", "arm64", "gira_v1.2.3_darwin_arm64.tar.gz"),
            ("Windows", "AMD64", "gira_v1.2.3_windows_amd64.zip"),
        ]
        for system, machine, expected in cases:
            with self.subTest(system=system, machine=machine):
                info = installer.resolve_platform(system, machine)
                self.assertEqual(installer.archive_name("v1.2.3", info), expected)

    def test_rejects_unsupported_platforms(self):
        with self.assertRaisesRegex(RuntimeError, "unsupported OS"):
            installer.resolve_platform("FreeBSD", "x86_64")
        with self.assertRaisesRegex(RuntimeError, "unsupported architecture"):
            installer.resolve_platform("Linux", "i386")
        with self.assertRaisesRegex(RuntimeError, "unsupported Windows architecture"):
            installer.resolve_platform("Windows", "arm64")

    def test_reads_checksum_entry_for_archive(self):
        checksums = "\n".join(
            [
                "abc123  gira_v1.2.3_linux_amd64.tar.gz",
                "def456 *gira_v1.2.3_darwin_arm64.tar.gz",
            ]
        )
        self.assertEqual(installer.checksum_for(checksums, "gira_v1.2.3_linux_amd64.tar.gz"), "abc123")
        self.assertEqual(installer.checksum_for(checksums, "gira_v1.2.3_darwin_arm64.tar.gz"), "def456")
        with self.assertRaisesRegex(RuntimeError, "does not include"):
            installer.checksum_for(checksums, "missing.tar.gz")

    def test_checksum_mismatch_fails_closed(self):
        with tempfile.TemporaryDirectory(prefix="gira-pypi-test-") as tmp:
            archive = Path(tmp) / "archive.tar.gz"
            archive.write_text("hello")
            with self.assertRaisesRegex(RuntimeError, "checksum mismatch"):
                installer.verify_checksum(archive, "0000")

    def test_checksum_match_passes(self):
        with tempfile.TemporaryDirectory(prefix="gira-pypi-test-") as tmp:
            archive = Path(tmp) / "archive.tar.gz"
            archive.write_text("hello")
            expected = hashlib.sha256(b"hello").hexdigest()
            installer.verify_checksum(archive, expected)

    def test_release_version_can_be_forced_by_environment(self):
        previous = os.environ.get("GIRA_VERSION")
        os.environ["GIRA_VERSION"] = "v9.8.7"
        try:
            self.assertEqual(installer.release_version(), "v9.8.7")
        finally:
            if previous is None:
                os.environ.pop("GIRA_VERSION", None)
            else:
                os.environ["GIRA_VERSION"] = previous

    def test_release_version_maps_package_version_to_tag(self):
        previous = os.environ.pop("GIRA_VERSION", None)
        try:
            with mock.patch.object(installer, "package_version", return_value="1.2.3"):
                self.assertEqual(installer.release_version(), "v1.2.3")
            with mock.patch.object(installer, "package_version", return_value="v1.2.3"):
                self.assertEqual(installer.release_version(), "v1.2.3")
            with mock.patch.object(installer, "package_version", return_value="0.0.0.dev0"):
                self.assertEqual(installer.release_version(), "")
        finally:
            if previous is not None:
                os.environ["GIRA_VERSION"] = previous

    def test_install_channel_from_wrapper_detects_uv_tool_path(self):
        paths = [
            "/home/me/.local/bin/gira",
            "/home/me/.local/share/uv/tools/gira-cli/bin/gira",
        ]
        self.assertEqual(installer.install_channel_from_wrapper(paths), "uv")

    def test_install_channel_from_wrapper_detects_pipx_path(self):
        paths = ["/home/me/.local/pipx/venvs/gira-cli/bin/gira"]
        self.assertEqual(installer.install_channel_from_wrapper(paths), "pipx")

    def test_native_environment_preserves_explicit_channel(self):
        previous = os.environ.get("GIRA_INSTALL_CHANNEL")
        os.environ["GIRA_INSTALL_CHANNEL"] = "homebrew"
        try:
            with mock.patch.object(installer, "candidate_wrapper_paths", return_value=[]):
                self.assertEqual(installer.native_environment()["GIRA_INSTALL_CHANNEL"], "homebrew")
        finally:
            if previous is None:
                os.environ.pop("GIRA_INSTALL_CHANNEL", None)
            else:
                os.environ["GIRA_INSTALL_CHANNEL"] = previous

    def test_native_environment_defaults_to_pip(self):
        previous = os.environ.pop("GIRA_INSTALL_CHANNEL", None)
        try:
            with mock.patch.object(installer, "candidate_wrapper_paths", return_value=[]):
                self.assertEqual(installer.native_environment()["GIRA_INSTALL_CHANNEL"], "pip")
        finally:
            if previous is not None:
                os.environ["GIRA_INSTALL_CHANNEL"] = previous

    def test_rejects_zip_path_traversal(self):
        with tempfile.TemporaryDirectory(prefix="gira-pypi-test-") as tmp:
            root = Path(tmp)
            archive = root / "bad.zip"
            with zipfile.ZipFile(archive, "w") as zip_file:
                zip_file.writestr("../outside", "bad")
            with self.assertRaisesRegex(RuntimeError, "unsafe archive member path"):
                installer.extract_archive(archive, "zip", root / "out")
            self.assertFalse((root / "outside").exists())

    def test_rejects_tar_path_traversal(self):
        with tempfile.TemporaryDirectory(prefix="gira-pypi-test-") as tmp:
            root = Path(tmp)
            archive = root / "bad.tar.gz"
            payload = root / "payload"
            payload.write_text("bad")
            with tarfile.open(archive, "w:gz") as tar_file:
                tar_file.add(payload, arcname="../outside")
            with self.assertRaisesRegex(RuntimeError, "unsafe archive member path"):
                installer.extract_archive(archive, "tar.gz", root / "out")
            self.assertFalse((root / "outside").exists())

    def test_cached_binary_requires_checksum_marker(self):
        with tempfile.TemporaryDirectory(prefix="gira-pypi-test-") as tmp:
            target = Path(tmp) / "gira"
            target.write_text("hello")
            target.chmod(0o755)
            self.assertFalse(installer.cached_binary_valid(target))

            installer.binary_checksum_path(target).write_text(f"{installer.file_checksum(target)}\n")
            self.assertTrue(installer.cached_binary_valid(target))

            target.write_text("corrupt")
            target.chmod(0o755)
            self.assertFalse(installer.cached_binary_valid(target))


if __name__ == "__main__":
    unittest.main()
