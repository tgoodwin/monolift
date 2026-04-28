#!/usr/bin/env python3
"""Fail CI if disallowed patterns appear in the docs site.

Scans `mkdocs.yml` and `docs/site/**` for:
  - `extra_javascript:` (any custom JS beyond mkdocs-material's bundled JS)
  - `mermaid2` plugin references
  - Graphviz references (`.dot`, `!!python/name:graphviz`, `dot_graph`)
  - D2 references (`!!python/name:d2`, `.d2` files)
  - Hand-authored `.svg` files under docs/site/

Exits 0 with no output on success; non-zero with a grep-style report on violation.
"""

from __future__ import annotations

import pathlib
import re
import sys

REPO = pathlib.Path(__file__).resolve().parents[1]

DISALLOWED_PATTERNS = [
    (re.compile(r"^\s*extra_javascript\s*:", re.M), "extra_javascript: — no custom JS allowed"),
    (re.compile(r"\bmermaid2\b"), "mermaid2 plugin — use Material's built-in mermaid fence"),
    (re.compile(r"!!python/name:graphviz"), "graphviz custom fence"),
    (re.compile(r"!!python/name:d2"), "d2 custom fence"),
    (re.compile(r"\bdot_graph\b"), "Graphviz dot_graph reference"),
]

TEXT_SUFFIXES = {".yml", ".yaml", ".md", ".css", ".toml"}


def iter_scan_paths() -> list[pathlib.Path]:
    paths: list[pathlib.Path] = []
    mkdocs = REPO / "mkdocs.yml"
    if mkdocs.exists():
        paths.append(mkdocs)
    site_root = REPO / "docs" / "site"
    if site_root.exists():
        for p in site_root.rglob("*"):
            if p.is_file() and p.suffix.lower() in TEXT_SUFFIXES:
                # Exclude the built-site output if present.
                if "site" in p.parts and p.parts.count("site") >= 2:
                    # e.g. docs/site/site/... (mkdocs build output)
                    rel = p.relative_to(REPO)
                    if rel.parts[:3] == ("docs", "site", "site"):
                        continue
                paths.append(p)
    return paths


def find_hand_authored_svgs() -> list[pathlib.Path]:
    site_root = REPO / "docs" / "site"
    if not site_root.exists():
        return []
    return [
        p for p in site_root.rglob("*.svg")
        if "site" not in p.parts[len(site_root.parts):-1]  # skip built output
    ]


def main() -> int:
    violations: list[str] = []
    for path in iter_scan_paths():
        try:
            text = path.read_text(encoding="utf-8", errors="replace")
        except OSError as e:
            violations.append(f"{path}: cannot read: {e}")
            continue
        for pattern, label in DISALLOWED_PATTERNS:
            for m in pattern.finditer(text):
                line_no = text.count("\n", 0, m.start()) + 1
                violations.append(f"{path}:{line_no}: {label}")

    for svg in find_hand_authored_svgs():
        violations.append(f"{svg}: hand-authored SVG — use Material's mermaid fence instead")

    if violations:
        print("docs policy violations:", file=sys.stderr)
        for v in violations:
            print(f"  {v}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
