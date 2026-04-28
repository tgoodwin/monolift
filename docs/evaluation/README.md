# Monolift Evaluation Index

Semantic index over `evaluation/` — real-world Go applications used as targets for
iterating on Monolift's compiler capabilities. Clones live under `evaluation/`
(gitignored); this directory tracks experiments, findings, and per-target dossiers.

## Targets

| # | Target | Domain | Upstream | Why interesting |
|---|--------|--------|----------|-----------------|
| [01](targets/01-gitea.md)      | **gitea**      | Git hosting / web app       | go-gitea/gitea          | Monolithic web app w/ heavy DB layer; repo-ops are natural lift candidates |
| [02](targets/02-mattermost.md) | **mattermost** | Team chat platform          | mattermost/mattermost   | Large codebase; websocket hubs, push, search are candidate lifts |
| [03](targets/03-caddy.md)      | **caddy**      | Web server / reverse proxy  | caddyserver/caddy       | Plugin-heavy; TLS/ACME + cert issuance are offload-shaped |
| [04](targets/04-listmonk.md)   | **listmonk**   | Newsletter / mailing lists  | knadh/listmonk          | Campaign workers + template rendering map cleanly to lifts |
| [05](targets/05-pocketbase.md) | **pocketbase** | Backend-as-a-service (SQLite) | pocketbase/pocketbase | Small, single-binary; good for end-to-end compiler tests |
| [06](targets/06-miniflux.md)   | **miniflux**   | RSS feed reader             | miniflux/v2             | Feed-fetch workers; bounded concurrency; small enough to fully lift |

## Per-target dossier structure

Each `targets/NN-<name>.md` contains:

1. **Snapshot** — pinned commit SHA, LOC, build command, entrypoint
2. **Architecture notes** — packages, ownership of state, concurrency model
3. **Lift candidates** — functions/packages flagged as offload candidates, with `evaluation/<name>/path/to/file.go:LINE` refs
4. **Experiments** — table of runs cross-referenced to `experiments/`
5. **Compiler-capability gaps** — things Monolift's compiler couldn't handle on this target (feeds back into core repo work)
6. **Blockers / open questions**

## Experiments

`experiments/YYYY-MM-DD-<slug>.md` — dated notes for each evaluation run.
Reference from target dossiers; keep raw logs/outputs out of git (they live
under `evaluation/<target>/` or `output/`).

## Reproducibility

`evaluation/MANIFEST.yaml` pins upstream URLs and commit SHAs for every target.
To reproduce a teammate's setup: `cd evaluation && <re-clone per manifest>`.
