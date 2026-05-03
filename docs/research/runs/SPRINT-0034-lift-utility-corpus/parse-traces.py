#!/usr/bin/env python3
"""Parse all M-*.synthesis.md files into per-candidate structured JSON files."""

import json
import re
import sys
from pathlib import Path

SPRINT_ROOT = Path(__file__).parent
PROJECTS_DIR = SPRINT_ROOT / "projects"
OUTPUT_DIR = SPRINT_ROOT / ".." / ".." / "activation-paths" / "traces"


def parse_synthesis(path: Path) -> dict | None:
    text = path.read_text()
    lines = text.splitlines()

    header_match = re.match(r"^#\s+(\S+/M-\d+):\s+(.+)$", lines[0])
    if not header_match:
        print(f"  WARN: no header match in {path}", file=sys.stderr)
        return None
    candidate_id = header_match.group(1)
    candidate_name = header_match.group(2).strip()

    region_root = None
    for line in lines:
        m = re.match(r"^Region root:\s+`([^`]+)`", line)
        if m:
            region_root = m.group(1)
            break

    steps = []
    in_trace_table = False
    for line in lines:
        if re.match(r"\|\s*Step\s*\|", line):
            in_trace_table = True
            continue
        if in_trace_table and re.match(r"\|[-|]+\|", line):
            continue
        if in_trace_table and line.startswith("|"):
            cols = [c.strip() for c in line.split("|")]
            cols = [c for c in cols if c]
            if len(cols) >= 4:
                step_str = cols[0]
                from_col = cols[1]
                to_col = cols[2]
                edge_type = cols[3]

                step_num = re.match(r"(\d+)", step_str)
                step_n = int(step_num.group(1)) if step_num else 0

                from_loc = extract_file_line(from_col)
                to_loc = extract_file_line(to_col)
                func_name = extract_func_name(to_col)
                edge_type_clean = edge_type.strip("`").strip()

                steps.append({
                    "step": step_n,
                    "from": from_loc,
                    "to": to_loc,
                    "func": func_name,
                    "edge_type": edge_type_clean,
                    "from_raw": from_col,
                    "to_raw": to_col,
                })
            else:
                in_trace_table = False
        elif in_trace_table and not line.startswith("|"):
            in_trace_table = False

    edge_summary = {}
    in_edge_table = False
    for line in lines:
        if re.match(r"\|\s*Edge type\s*\|", line):
            in_edge_table = True
            continue
        if in_edge_table and re.match(r"\|[-|]+\|", line):
            continue
        if in_edge_table and line.startswith("|"):
            cols = [c.strip() for c in line.split("|")]
            cols = [c for c in cols if c]
            if len(cols) >= 3:
                et = cols[0].strip("`").strip()
                count = cols[1].strip()
                try:
                    edge_summary[et] = int(count)
                except ValueError:
                    edge_summary[et] = count
        elif in_edge_table and not line.startswith("|"):
            in_edge_table = False

    path_length = None
    hardest_edge = None
    fully_resolvable = None
    for line in lines:
        m = re.match(r"-\s+\*\*Path length:\*\*\s+(\d+)", line)
        if m:
            path_length = int(m.group(1))
        m = re.match(r"-\s+\*\*Hardest edge:\*\*\s+(.+)", line)
        if m:
            hardest_edge = m.group(1).strip()
        m = re.match(r"-\s+\*\*Fully statically resolvable:\*\*\s+(\S+)", line)
        if m:
            fully_resolvable = m.group(1).strip().rstrip("—").strip()

    return {
        "id": candidate_id,
        "name": candidate_name,
        "region_root": region_root,
        "path_length": path_length,
        "fully_resolvable": fully_resolvable,
        "hardest_edge": hardest_edge,
        "steps": steps,
        "edge_summary": edge_summary,
    }


def extract_file_line(col: str) -> str | None:
    m = re.search(r"`([^`]*?:\d+)`", col)
    if m:
        return m.group(1)
    m = re.search(r"(\S+\.\w+:\d+)", col)
    if m:
        return m.group(1)
    return None


def extract_func_name(col: str) -> str | None:
    m = re.search(r"`([^`]+)`\s+`([^`]+)`", col)
    if m:
        return m.group(2)
    m = re.search(r":\d+`\s+`?(\w[\w.*()]+)`?", col)
    if m:
        return m.group(1)
    return None


def main():
    OUTPUT_DIR.mkdir(parents=True, exist_ok=True)
    projects = sorted(d.name for d in PROJECTS_DIR.iterdir() if d.is_dir())

    total = 0
    all_edge_types = {}

    for project in projects:
        traces_dir = PROJECTS_DIR / project / "traces"
        if not traces_dir.exists():
            continue
        synth_files = sorted(traces_dir.glob("M-*.synthesis.md"))
        for sf in synth_files:
            m_num = re.match(r"(M-\d+)", sf.stem)
            if not m_num:
                continue

            result = parse_synthesis(sf)
            if not result:
                continue

            result["project"] = project
            result["source_file"] = str(sf.relative_to(SPRINT_ROOT))

            # Write per-candidate file: caddy-M-1.json, gitea-M-14.json, etc.
            out_name = f"{project}-{m_num.group(1)}.json"
            out_path = OUTPUT_DIR / out_name
            out_path.write_text(json.dumps(result, indent=2) + "\n")

            total += 1
            for s in result["steps"]:
                et = s["edge_type"]
                all_edge_types[et] = all_edge_types.get(et, 0) + 1

            print(f"  {result['id']}: {len(result['steps'])} steps -> {out_name}", file=sys.stderr)

    print(f"\nWrote {total} trace files to {OUTPUT_DIR}", file=sys.stderr)
    total_steps = sum(all_edge_types.values())
    print(f"Total steps: {total_steps}", file=sys.stderr)
    print(f"Distinct edge types: {len(all_edge_types)}", file=sys.stderr)
    for et, count in sorted(all_edge_types.items(), key=lambda x: -x[1])[:10]:
        print(f"  {count:3d}  {et}", file=sys.stderr)


if __name__ == "__main__":
    main()
