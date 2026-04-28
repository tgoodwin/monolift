#!/usr/bin/env python3
"""Verify every `--8<-- "path:Lstart:Lend"` include matches its marker pair.

For each include whose target file lives under `pkg/compiler/**` (Monolift
source), look for matching `// site:begin NAME` / `// site:end NAME` comments
in the target file. The marker NAME is the include's `local_name` as stated
in a sibling YAML entry, OR — more directly — matched by an implicit name
convention: the include may declare the name via an HTML comment on the
preceding Markdown line, e.g.:

    <!-- site-anchor: canonical-shapes-classifier -->
    ```go
    --8<-- "pkg/compiler/shape/shape.go:177:223"
    ```

If the marker span (line-after-`begin NAME` through line-before-`end NAME`)
does not equal the include's `Lstart:Lend`, emit a diff-style message and
exit non-zero.

Vendored external snippets (`docs/site/snippets/external/**/*.go.txt`) are
not checked here — external drift is enforced by refresh-external-snippets.py
+ `git diff --exit-code` in CI.
"""

from __future__ import annotations

import pathlib
import re
import sys
from dataclasses import dataclass

REPO = pathlib.Path(__file__).resolve().parents[1]
DOCS_GLOB = REPO / "docs" / "site" / "docs"

INCLUDE_RE = re.compile(
    r'--8<--\s+"([^":]+):(\d+):(\d+)"'
)
ANCHOR_RE = re.compile(r"<!--\s*site-anchor:\s*([A-Za-z0-9_-]+)\s*-->")


@dataclass
class Include:
    md_file: pathlib.Path
    md_line: int
    target: pathlib.Path
    l_start: int
    l_end: int
    anchor: str | None


def iter_includes() -> list[Include]:
    out: list[Include] = []
    if not DOCS_GLOB.exists():
        return out
    for md in DOCS_GLOB.rglob("*.md"):
        text = md.read_text(encoding="utf-8")
        lines = text.splitlines()
        current_anchor: str | None = None
        for i, line in enumerate(lines, start=1):
            m_anchor = ANCHOR_RE.search(line)
            if m_anchor:
                current_anchor = m_anchor.group(1)
                continue
            m_inc = INCLUDE_RE.search(line)
            if m_inc:
                target_rel = m_inc.group(1)
                target = (REPO / target_rel).resolve()
                out.append(Include(
                    md_file=md,
                    md_line=i,
                    target=target,
                    l_start=int(m_inc.group(2)),
                    l_end=int(m_inc.group(3)),
                    anchor=current_anchor,
                ))
                current_anchor = None
    return out


def find_marker_span(target: pathlib.Path, anchor: str) -> tuple[int, int] | None:
    """Return (inclusive) line numbers between site:begin NAME and site:end NAME."""
    if not target.exists():
        return None
    begin_re = re.compile(rf"^\s*//\s*site:begin\s+{re.escape(anchor)}\s*$")
    end_re = re.compile(rf"^\s*//\s*site:end\s+{re.escape(anchor)}\s*$")
    begin_line: int | None = None
    end_line: int | None = None
    for i, line in enumerate(target.read_text(encoding="utf-8").splitlines(), start=1):
        if begin_re.match(line):
            begin_line = i
        elif end_re.match(line):
            end_line = i
            break
    if begin_line is None or end_line is None:
        return None
    # Span inside the markers: first line after begin through last line before end.
    return (begin_line + 1, end_line - 1)


def main() -> int:
    includes = iter_includes()
    errors: list[str] = []
    for inc in includes:
        # Only check Monolift-source includes here.
        try:
            rel = inc.target.relative_to(REPO)
        except ValueError:
            errors.append(f"{inc.md_file}:{inc.md_line}: include target is outside repo: {inc.target}")
            continue
        if not str(rel).startswith("pkg/compiler/"):
            continue
        if inc.anchor is None:
            errors.append(
                f"{inc.md_file}:{inc.md_line}: Monolift-source include lacks a preceding "
                f"`<!-- site-anchor: NAME -->` marker comment"
            )
            continue
        span = find_marker_span(inc.target, inc.anchor)
        if span is None:
            errors.append(
                f"{inc.md_file}:{inc.md_line}: no matching `// site:begin {inc.anchor}` / "
                f"`// site:end {inc.anchor}` pair found in {rel}"
            )
            continue
        expected_start, expected_end = span
        if (inc.l_start, inc.l_end) != (expected_start, expected_end):
            errors.append(
                f"{inc.md_file}:{inc.md_line}: drift on anchor `{inc.anchor}` in {rel}:\n"
                f"    include range : L{inc.l_start}-L{inc.l_end}\n"
                f"    marker span   : L{expected_start}-L{expected_end}\n"
                f"    fix           : scripts/fix-snippet-drift.py"
            )
    if errors:
        print("snippet drift detected:", file=sys.stderr)
        for e in errors:
            print(f"  {e}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
