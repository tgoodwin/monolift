# Critique of gitea drafts by gemini

## Verdicts on claude's draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| C-1 (Webhook delivery worker) | KEEP | rubric criterion satisfied: Compute envelope. Matches Gemini G-5 and Codex C-1. This is a classic async worker that aggregates well under fan-out and handles non-trivial payload signing and delivery. |
| C-2 (Repository archive generator) | KEEP | rubric criterion satisfied: Compute envelope. Matches Gemini G-6 and Codex C-2. Highly CPU and IO intensive work that scales with repository size and is already isolated via a queue. |
| C-3 (Mirror pull sync) | KEEP | rubric criterion satisfied: Variable / spikable load. Mirroring many repositories can lead to massive spikes in network and Git activity, making it a prime candidate for offloading despite disk-locality risks. |
| C-4 (Code indexer) | KEEP | rubric criterion satisfied: Compute envelope. Matches Gemini G-10 and Codex C-6. The heavy work of diffing, blob reading, and language detection before indexing is a significant CPU consumer. |
| C-5 (Issue indexer) | KEEP | rubric criterion satisfied: Coherent unit. Well-isolated per-item indexing task that helps offload search-related load from the primary request paths. |
| C-6 (Mailer queue worker) | KEEP | rubric criterion satisfied: Latency tolerance and failure model. Standard async task that benefits from independent scaling and doesn't block user-facing requests. |
| C-7 (PR merge-and-push) | KEEP | rubric criterion satisfied: Compute envelope. Involves complex Git orchestrations and temporary repository management, making it one of the heaviest operations in Gitea. |
| C-8 (Repository migration) | KEEP | rubric criterion satisfied: Compute envelope. Matches Codex C-4. Often involves minutes of outbound network and Git processing; offloading this keeps the main process responsive. |
| C-9 (Avatar image processing) | KEEP | rubric criterion satisfied: State independence. Matches Gemini G-3. A pure functional transformation that is CPU and memory intensive, perfectly suited for lifting. |
| C-10 (Diff parsing) | KEEP | rubric criterion satisfied: Compute envelope. Matches Gemini G-2. Parsing large unified diffs is a significant CPU and allocation burden during PR and commit views. |
| C-11 (Push-update worker) | KEEP | rubric criterion satisfied: Variable / spikable load. Although complex, it anchors the cascade of downstream effects (hooks, indexing) triggered by pushes, which are the primary load driver. |
| C-12 (Markdown render) | KEEP | rubric criterion satisfied: Compute envelope. Matches Gemini G-1 and Codex C-10. While Claude correctly notes it can be marginal for small inputs, it is a major CPU sink for large documents and scales linearly with payload. |

## Verdicts on codex's draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| C-1 (Webhook) | KEEP | rubric criterion satisfied: Compute envelope. Essential async task, matches G-5/C-1. |
| C-2 (Archive) | KEEP | rubric criterion satisfied: Compute envelope. Heavy background work, matches G-6/C-2. |
| C-3 (PR mergeability check) | KEEP | rubric criterion satisfied: Variable / spikable load. Excellent pick; these checks are frequent, CPU-bound, and can be triggered in batches on base branch updates. |
| C-4 (Migration) | KEEP | rubric criterion satisfied: Compute envelope. Heavy long-running task, matches C-8. |
| C-5 (Mirror) | KEEP | rubric criterion satisfied: Compute envelope. Spikable background task, matches C-3. |
| C-6 (Code indexing) | KEEP | rubric criterion satisfied: Compute envelope. Matches G-10/C-4. |
| C-7 (Mirror LFS sync) | KEEP | rubric criterion satisfied: Compute envelope. High IO/Compute mix specifically for large binary assets; valuable specialization of the mirror sync. |
| C-8 (RPM metadata rebuild) | KEEP | rubric criterion satisfied: Compute envelope. Great example of a subsystem-specific spikable task involving XML generation and crypto signing. |
| C-9 (Actions workflow detection) | KEEP | rubric criterion satisfied: Compute envelope. Involves recursive filesystem listing and YAML parsing across many files, creating meaningful CPU load per event. |
| C-10 (Markdown rendering) | KEEP | rubric criterion satisfied: Compute envelope. Matches G-1/C-12. Correct function root cited at `:186`. |
| C-11 (Git diff construction) | KEEP | rubric criterion satisfied: Compute envelope. Matches G-2/C-10 but includes the Git execution phase, which is even more meaningful to lift. |
| C-12 (NPM upload parsing) | DROP | rubric criterion fails: Compute envelope. The utility of offloading SHA512 and Base64 decoding is likely outweighed by the latency and bandwidth cost of moving the entire package payload (tens of MBs) to a remote worker. |

## Overlooked

### O-1: Debian Repository Metadata Rebuild

- **Region root:** `evaluation/gitea/services/packages/debian/repository.go:154` — `BuildSpecificRepositoryFiles` rebuilds Debian package indexes.
- **Caller(s):** `evaluation/gitea/routers/api/packages/debian/debian.go:121` (after upload), `:204` (after delete).
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Walks package database, generates Gzipped Package/Release files, and performs PGP signing.
  - Load profile: yes — Triggered by package uploads; cost scales with total package count in the distribution/component.
  - Coherent unit: yes — Clean input parameters (owner, distribution, component, arch).
  - State independence: yes — Reads/writes go through package storage models.
  - Latency / failure: maybe — Synchronous on the upload path, but the work is substantial enough to warrant isolation.
- **Activation shape:** HTTP route post-processing.
- **Confidence:** high — Mirror of the RPM candidate (Codex C-8) and equally valid for large package registries.
- **Risk notes:** Requires access to signing keys and shared package storage.

## Overall observations

The three drafts show high convergence on the "Big 6" computational tasks (Webhook, Archive, Indexer, Avatar, Markdown, and Diff). Claude provided the most thorough investigation of background worker patterns and migration services, while Codex identified valuable niche candidates in the Actions and Package subsystems. Gemini's draft (anchor) focused most heavily on pure computational transforms like Argon2 and Syntax Highlighting. Codex was slightly less rigorous on the compute envelope for C-12, where the data-transfer overhead likely negates the lift benefit.
