#!/usr/bin/env python3
"""Refresh vendored external snippets from pinned upstream SHAs.

Reads:
  - evaluation/MANIFEST.yaml  (upstream URL + SHA per target)
  - docs/site/snippets/external/excerpts.yaml  (per-file entries)

For each entry: fetches the upstream repo at its pinned SHA using
`git archive` against a shallow bare mirror under `.cache/evaluation-snippets/`
(CI-recreatable, not read from the gitignored `evaluation/` clones), slices
the requested line range, prepends a provenance header, and writes to
`docs/site/snippets/external/<project>/<local_name>.go.txt`.

Idempotent: re-running produces byte-identical output when inputs are
unchanged. CI verifies this via `git diff --exit-code` after a refresh pass.
"""

from __future__ import annotations

import datetime as _dt
import pathlib
import subprocess
import sys

try:
    import yaml
except ImportError as e:
    print("this script requires PyYAML (install from docs/site/requirements.txt)", file=sys.stderr)
    raise

REPO = pathlib.Path(__file__).resolve().parents[1]
MANIFEST = REPO / "evaluation" / "MANIFEST.yaml"
EXCERPTS = REPO / "docs" / "site" / "snippets" / "external" / "excerpts.yaml"
OUT_ROOT = REPO / "docs" / "site" / "snippets" / "external"
CACHE_ROOT = REPO / ".cache" / "evaluation-snippets"

TODAY = _dt.date.today().isoformat()


def load_manifest() -> dict[str, dict]:
    data = yaml.safe_load(MANIFEST.read_text(encoding="utf-8"))
    return {t["name"]: t for t in data.get("targets", [])}


def load_excerpts() -> list[dict]:
    if not EXCERPTS.exists():
        return []
    data = yaml.safe_load(EXCERPTS.read_text(encoding="utf-8"))
    return data or []


def ensure_bare_clone(target: dict) -> pathlib.Path:
    """Ensure a bare mirror exists under the cache; return its path."""
    CACHE_ROOT.mkdir(parents=True, exist_ok=True)
    bare = CACHE_ROOT / f"{target['name']}.git"
    if not bare.exists():
        subprocess.check_call(
            ["git", "clone", "--mirror", target["upstream"], str(bare)],
            stdout=subprocess.DEVNULL, stderr=subprocess.STDOUT,
        )
    # Fetch the pinned SHA if it's not present (cheap when already present).
    subprocess.check_call(
        ["git", "-C", str(bare), "fetch", "--quiet", "origin", target["sha"]],
        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    return bare


def archive_file(bare: pathlib.Path, sha: str, upstream_path: str) -> str:
    """Return the full file content at the given SHA/path."""
    out = subprocess.check_output(
        ["git", "-C", str(bare), "show", f"{sha}:{upstream_path}"],
        text=True,
    )
    return out


def slice_range(content: str, l_start: int, l_end: int) -> str:
    lines = content.splitlines(keepends=True)
    if l_start < 1 or l_end > len(lines) or l_start > l_end:
        raise ValueError(f"invalid range L{l_start}-L{l_end} for file of {len(lines)} lines")
    sliced = "".join(lines[l_start - 1:l_end])
    if not sliced.endswith("\n"):
        sliced += "\n"
    return sliced


def provenance_header(target: dict, entry: dict) -> str:
    # Strip any trailing `.git` for a human-readable upstream ref.
    upstream = target["upstream"]
    if upstream.endswith(".git"):
        upstream = upstream[:-4]
    l_start, l_end = entry["upstream_range"].split("-")
    return (
        f"// source: {upstream}@{target['sha']}\n"
        f"// path:   {entry['upstream_path']}\n"
        f"// lines:  L{l_start}-L{l_end}\n"
        f"// SPDX-License-Identifier: {entry['spdx']}\n"
        f"// fetched: {TODAY}\n"
        f"// rationale: {entry['rationale']}\n"
        "\n"
    )


def main() -> int:
    targets = load_manifest()
    excerpts = load_excerpts()
    if not excerpts:
        print("no external excerpts defined; nothing to refresh.")
        return 0

    errors: list[str] = []
    for entry in excerpts:
        project = entry["project"]
        if project not in targets:
            errors.append(f"excerpts.yaml: project {project!r} not in MANIFEST.yaml")
            continue
        target = targets[project]
        try:
            bare = ensure_bare_clone(target)
        except subprocess.CalledProcessError as e:
            errors.append(f"{project}: clone/fetch failed: {e}")
            continue
        try:
            full = archive_file(bare, target["sha"], entry["upstream_path"])
        except subprocess.CalledProcessError as e:
            errors.append(f"{project}:{entry['upstream_path']}: show failed: {e}")
            continue
        l_start, l_end = (int(x) for x in entry["upstream_range"].split("-"))
        try:
            body = slice_range(full, l_start, l_end)
        except ValueError as e:
            errors.append(f"{project}:{entry['upstream_path']}: {e}")
            continue
        out_dir = OUT_ROOT / project
        out_dir.mkdir(parents=True, exist_ok=True)
        out_path = out_dir / f"{entry['local_name']}.go.txt"
        # Header uses a deterministic `fetched:` field that won't cause diff
        # churn on re-runs UNLESS the manifest changes. To make this stable
        # across days, prefer the MANIFEST.yaml `pinned_at` date if available.
        pinned_at = _manifest_pinned_at()
        fetched_line_date = pinned_at or TODAY
        header = provenance_header(target, entry).replace(
            f"// fetched: {TODAY}",
            f"// fetched: {fetched_line_date}",
        )
        out_path.write_text(header + body, encoding="utf-8")
        print(f"wrote {out_path.relative_to(REPO)} ({len(body.splitlines())} lines)")

    if errors:
        print("errors:", file=sys.stderr)
        for e in errors:
            print(f"  {e}", file=sys.stderr)
        return 1
    return 0


def _manifest_pinned_at() -> str | None:
    try:
        data = yaml.safe_load(MANIFEST.read_text(encoding="utf-8"))
    except OSError:
        return None
    v = data.get("pinned_at")
    if isinstance(v, _dt.date):
        return v.isoformat()
    if isinstance(v, str):
        return v
    return None


if __name__ == "__main__":
    sys.exit(main())
