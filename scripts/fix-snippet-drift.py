#!/usr/bin/env python3
"""Rewrite `--8<--` include ranges to match their `site:begin/end` markers.

Safe-mode: refuses to run if the working tree has uncommitted changes to files
other than the ones this script would touch.

Usage:
    python3 scripts/fix-snippet-drift.py            # apply fixes
    python3 scripts/fix-snippet-drift.py --dry-run  # print what would change
"""

from __future__ import annotations

import argparse
import pathlib
import subprocess
import sys

# Reuse helpers from check-snippet-drift.
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))
from importlib import import_module

_check = import_module("check-snippet-drift")

REPO = pathlib.Path(__file__).resolve().parents[1]


def working_tree_dirty_outside(touch_paths: set[pathlib.Path]) -> list[str]:
    """Return a list of dirty file paths that are NOT in touch_paths."""
    try:
        out = subprocess.check_output(
            ["git", "-C", str(REPO), "status", "--porcelain"],
            text=True,
        )
    except (subprocess.CalledProcessError, FileNotFoundError):
        return []
    dirty: list[str] = []
    touch_rel = {str(p.relative_to(REPO)) for p in touch_paths}
    for line in out.splitlines():
        if not line.strip():
            continue
        # Porcelain format: "XY path" (two status chars + space + path).
        path = line[3:].strip()
        # Handle renames "old -> new".
        if " -> " in path:
            path = path.split(" -> ", 1)[1]
        if path not in touch_rel:
            dirty.append(path)
    return dirty


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()

    includes = _check.iter_includes()
    fixes: list[tuple[pathlib.Path, int, str, tuple[int, int], tuple[int, int]]] = []

    for inc in includes:
        try:
            rel = inc.target.relative_to(REPO)
        except ValueError:
            continue
        if not str(rel).startswith("pkg/compiler/"):
            continue
        if inc.anchor is None:
            continue
        span = _check.find_marker_span(inc.target, inc.anchor)
        if span is None:
            continue
        if (inc.l_start, inc.l_end) != span:
            fixes.append((inc.md_file, inc.md_line, inc.anchor, (inc.l_start, inc.l_end), span))

    if not fixes:
        print("no drift to fix.")
        return 0

    touch_files = {f[0] for f in fixes}
    if not args.dry_run:
        dirty = working_tree_dirty_outside(touch_files)
        if dirty:
            print("refusing to run: working tree has unrelated uncommitted changes:", file=sys.stderr)
            for d in dirty:
                print(f"  {d}", file=sys.stderr)
            print("commit or stash those changes first, then re-run.", file=sys.stderr)
            return 2

    for md_file, md_line, anchor, (old_s, old_e), (new_s, new_e) in fixes:
        text = md_file.read_text(encoding="utf-8")
        lines = text.splitlines(keepends=True)
        idx = md_line - 1
        line = lines[idx]
        old_token = f'"{_target_in(line)}:{old_s}:{old_e}"'
        new_token = f'"{_target_in(line)}:{new_s}:{new_e}"'
        lines[idx] = line.replace(old_token, new_token)
        print(f"{md_file.relative_to(REPO)}:{md_line}  "
              f"[{anchor}]  L{old_s}-L{old_e} -> L{new_s}-L{new_e}")
        if not args.dry_run:
            md_file.write_text("".join(lines), encoding="utf-8")

    return 0


def _target_in(line: str) -> str:
    import re
    m = re.search(r'--8<--\s+"([^":]+):\d+:\d+"', line)
    if not m:
        raise RuntimeError(f"could not parse include on line: {line!r}")
    return m.group(1)


if __name__ == "__main__":
    sys.exit(main())
