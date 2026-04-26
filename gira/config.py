from __future__ import annotations

from dataclasses import dataclass


@dataclass(frozen=True)
class RepoRef:
    owner: str
    name: str

    @property
    def full_name(self) -> str:
        return f"{self.owner}/{self.name}"


def parse_repo_ref(value: str) -> RepoRef:
    """Parse OWNER/REPO into a RepoRef."""
    parts = value.strip().split("/")
    if len(parts) != 2 or not all(parts):
        raise ValueError("repo must be in OWNER/REPO format")
    return RepoRef(owner=parts[0], name=parts[1])
