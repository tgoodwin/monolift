# Contributing to the Monolift design-story site

This site is an educational companion to the paper and the ADR trail.
It is deliberately small and has strict discipline about what goes in
and how snippets stay in sync with code.

## Local setup

```bash
python3 -m venv ~/.virtualenvs/monolift
source ~/.virtualenvs/monolift/bin/activate
pip install -r docs/site/requirements.txt
```

Then:

- `make docs-site-serve` — live preview at `http://127.0.0.1:8000/monolift/`.
- `make docs-site-build` — one-shot `mkdocs build --strict`.
- `make docs-site-verify` — policy + drift + refresh-no-op (what CI runs).

## Adding a section to an existing page

Prose is capped at one opening paragraph + one mermaid diagram + one
side-by-side pairing + a ≤3-sentence closing "Why we did this" linking
ADRs. If a section wants to grow beyond that, it belongs in an ADR or
in `docs/evolution.md` — not on the site.

## Anchoring a Monolift snippet

1. In the target `.go` file under `pkg/compiler/**`, wrap the excerpt
   with matched comments on their own lines:
   ```go
   // site:begin my-anchor-name
   func myThing(...) {...}
   // site:end my-anchor-name
   ```
   Pick a unique, descriptive kebab-case name.
2. In the Markdown page, precede the include with a marker comment:
   ```markdown
   <!-- site-anchor: my-anchor-name -->
   ```go
   --8<-- "pkg/compiler/pkg/file.go:LINE_BEGIN:LINE_END"
   ```
   ```
   The include range is **the body of the excerpt** — the line after
   `site:begin` through the line before `site:end`, inclusive.
3. Run `make docs-site-check-drift`. If the range is wrong, run
   `python3 scripts/fix-snippet-drift.py` to rewrite it to match the
   marker span.

## Adding a vendored external excerpt

1. Find the exact line range in the upstream target. The gitignored
   `evaluation/<project>/` clones are convenient for this — but CI
   does not read from them, so they are authoring-only.
2. Add an entry to `docs/site/snippets/external/excerpts.yaml`.
   Required fields (shape defined in `excerpts.schema.yaml`):
   `project`, `upstream_path`, `upstream_range`, `local_name`,
   `rationale` (≤120 chars), `spdx`. `project` must match a `name`
   in `evaluation/MANIFEST.yaml`.
3. Run `make docs-site-refresh`. The script fetches the upstream
   content at the pinned SHA via `git show` against a cached bare
   mirror under `.cache/evaluation-snippets/`, slices the line range,
   prepends a provenance header (upstream URL + SHA + path + range +
   SPDX + fetch date), and writes
   `docs/site/snippets/external/<project>/<local_name>.go.txt`.
4. Commit both the manifest entry and the generated `.go.txt`.
5. Reference it from Markdown with a normal `pymdownx.snippets` include:
   ```markdown
   ```go
   --8<-- "docs/site/snippets/external/<project>/<local_name>.go.txt"
   ```
   ```

## Bumping a pinned SHA in `evaluation/MANIFEST.yaml`

1. Update the SHA. Update `pinned_at` to today's date.
2. Run `make docs-site-refresh`. Every vendored excerpt for that
   project regenerates; some may shift line numbers or fail range
   validation.
3. Fix each excerpt's `upstream_range` in `excerpts.yaml` and
   re-run until the refresh is a clean no-op.
4. Commit both the manifest bump and the updated `.go.txt` files.
   CI will fail on PRs that commit one without the other.

## Drift-check semantics

- **Monolift-source drift** (`scripts/check-snippet-drift.py`):
  for each `--8<-- "pkg/compiler/*"` include, resolves the preceding
  `<!-- site-anchor: NAME -->` to the matching `site:begin/end NAME`
  marker pair in the target file; fails if the include's line range
  does not equal the marker span (begin+1 through end-1 inclusive).
  Emits a copy-pasteable diff and a pointer to `fix-snippet-drift.py`.
- **External snippet drift**: enforced by re-running
  `scripts/refresh-external-snippets.py` in CI and failing on any
  non-empty `git diff -- docs/site/snippets/external/`. No separate
  check script.
- **Policy drift** (`scripts/check-docs-policy.py`): grep-scans for
  `extra_javascript:`, non-Material mermaid plugins, Graphviz/D2
  references, and hand-authored SVG in `mkdocs.yml` and `docs/site/`.
  Introducing any of these fails the build. See the script for the
  exact regex list.

## First-time Pages enablement (one-time manual step)

After this workflow first runs on `main`, an author must:

1. Open the repo's GitHub Settings → Pages.
2. Under "Build and deployment" → "Source", select **GitHub Actions**.
3. Confirm that subsequent pushes to `main` trigger the `deploy` job
   and that the published site is reachable at
   `https://tgoodwin.github.io/monolift/`.

This cannot be automated via the workflow; it is recorded here so the
step is not rediscovered every time.
