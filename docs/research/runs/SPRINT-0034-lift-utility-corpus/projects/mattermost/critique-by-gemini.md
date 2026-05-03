# Critique of mattermost drafts by gemini

## Verdicts on claude's draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| C-1 (docextractor.Extract) | KEEP | rubric criterion satisfied: Compute envelope. Heavily CPU-bound parsing of complex binary formats. Claude's focus on the `docextractor` package provides a more surgical implementation boundary than my own C-2. |
| C-2 (Attachment image post-processing) | KEEP | rubric criterion satisfied: Compute envelope. Classic CPU-bound media processing (decode/resize/encode). Already largely asynchronous in the current architecture. |
| C-3 (Outgoing-webhook delivery) | KEEP | rubric criterion satisfied: Load profile. Spiky fan-out to external URLs with significant aggregation potential. Correctly identifies the bursty nature of chat-driven integrations. |
| C-4 (Link-preview metadata fetch) | KEEP | rubric criterion satisfied: Latency tolerance. Inherently high-latency due to outbound HTTP; adding a network hop is negligible. Matches my C-3. |
| C-5 (Recap channel processing) | KEEP | rubric criterion satisfied: Compute envelope. Orchestrates expensive LLM calls and content enrichment. Fits the "periodic but heavy" bucket for tenant-scale summarization. |
| C-6 (Batched email notification) | KEEP | rubric criterion satisfied: Latency tolerance. Background job worker with high latency tolerance. Better scoped than my C-5 by focusing on the batching logic. |
| C-7 (Incoming webhook ingestion) | DROP | fails rubric criterion: Coherent unit. As Claude notes, this is heavily coupled to the `CreatePost` pipeline, which drags in a significant portion of the monolith's core state and logic. |
| C-8 (Slack workspace import) | KEEP | rubric criterion satisfied: Compute envelope. Massive one-off task involving heavy JSON parsing and image processing. Perfect candidate for offloading during admin operations. |
| C-9 (Per-post indexing) | MODIFY | Incorrect framing: "Per-post" indexing is often too thin to justify a lift if it's just a JSON marshal. Recommend merging with or deferring to Codex's C-7, which handles batch indexing at the worker level. |
| C-10 (Bulk team export) | KEEP | rubric criterion satisfied: Compute envelope. Significant IO and serialization work proportional to workspace size. Fits the "periodic but heavy" criteria. |

## Verdicts on codex's draft

| Candidate ID | Verdict | One-paragraph reasoning |
|---|---|---|
| C-1 (File content extraction) | KEEP | rubric criterion satisfied: Compute envelope. Matches my C-2 and Claude's C-1. Verified as a high-value CPU-bound target. |
| C-2 (Image upload processing) | KEEP | rubric criterion satisfied: Compute envelope. Matches my C-1 and Claude's C-2. Highly spiky and compute-intensive. |
| C-3 (Remote cluster transfer) | KEEP | rubric criterion satisfied: Latency tolerance. Already an isolated queue-driven task with clear failure modes and no synchronous user-facing impact. |
| C-4 (Outgoing webhook fan-out) | KEEP | rubric criterion satisfied: Load profile. Matches Claude C-3. Excellent for offloading outbound IO pressure. |
| C-5 (Bulk import processing) | KEEP | rubric criterion satisfied: Compute envelope. Generalization of Claude's C-8; handles massive tenant-scale data ingestion in the background. |
| C-6 (Bulk export archive) | KEEP | rubric criterion satisfied: Compute envelope. Matches Claude C-10. Heavy background serialization task. |
| C-7 (Elasticsearch bulk indexing) | KEEP | rubric criterion satisfied: Compute envelope. Superior to Claude C-9 by targeting the batch-oriented `IndexerWorker`, which has a cleaner compute-to-overhead ratio. |
| C-8 (Post search request) | DROP | Alternative already in my draft: C-6. Codex marks this as "medium" confidence due to latency sensitivity; my C-6 scoring covers the same trade-offs. |
| C-9 (File search request) | KEEP | rubric criterion satisfied: Compute envelope. Similar logic to post search but involves potentially heavy file metadata and content queries. |
| C-10 (Custom slash command) | KEEP | rubric criterion satisfied: State independence. Integration-heavy path with a clean contract and high latency tolerance for the caller. |
| C-11 (Push notification fan-out) | KEEP | rubric criterion satisfied: Load profile. Spiky, per-session fan-out with mandatory JWT signing work. Fits the "bursty" criteria well. |
| C-12 (Email notification render) | KEEP | rubric criterion satisfied: Latency tolerance. Matches my C-5 and Claude's C-6. Classic background compute task. |

## Overlooked

### C-13: LDAP User/Group Synchronization

- **Region root:** `evaluation/mattermost/server/channels/app/ldap.go:20` — `App.SyncLdap` (via `ldapI.StartSynchronizeJob`) orchestrates the synchronization of thousands of users and groups from external LDAP/AD.
- **Caller(s):** `evaluation/mattermost/server/channels/app/ldap.go:20` — triggered by admin manual sync or scheduled job.
- **Why useful (rubric scoring):**
  - Compute envelope: yes — Heavy IO-bound work (LDAP queries) and CPU-bound work (diffing large result sets against internal Store, updating memberships).
  - Load profile: yes — Periodic but extremely heavy; can dominate system resources during sync of large enterprise directories.
  - Coherent unit: yes — Entry point is clean; the job worker encapsulates the logic.
  - State independence: yes — Reads from external LDAP and writes to durable Stores.
  - Latency / failure: yes — Fully background task; failure is logged and the job is retried by the scheduler.
- **Activation shape:** Background job worker launched from goroutine.
- **Confidence:** high — A classic "periodic but heavy" task that protects the core monolith from LDAP latency and processing overhead.
- **Risk notes:** Requires access to the same LDAP configuration and credentials as the main process.

## Overall observations

The three drafts converge cleanly on file and image processing as the highest-value targets. Claude and Codex both correctly identified integration fan-out (webhooks, previews) which I missed in my Phase 1, while my draft was unique in identifying low-level crypto (password hashing) and analytics. Codex provided the most rigorous mapping of background job workers (Push, Remote cluster), which often have the cleanest "lift" boundaries.
