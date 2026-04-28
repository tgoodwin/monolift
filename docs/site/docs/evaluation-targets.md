# Evaluation targets

The compiler is developed against a pinned set of real open-source Go
monoliths, not a synthetic benchmark. Each project represents a design
pressure the compiler has to answer: frameworks with non-standard
handler signatures, methods bound to stateful receivers, embedded
databases, large internal call graphs, long-running workers. The full
list is pinned in [`evaluation/MANIFEST.yaml`](https://github.com/tgoodwin/monolift/blob/main/evaluation/MANIFEST.yaml)
with a commit SHA per target; the table below is the reader-facing
summary.

| Project | Repository | Go LOC | What it is |
|---|---|---:|---|
| **Caddy** | [caddyserver/caddy](https://github.com/caddyserver/caddy) | 93.5k | Extensible open-source web server with automatic HTTPS. |
| **Gitea** | [go-gitea/gitea](https://github.com/go-gitea/gitea) | 455.9k | Self-hosted Git service with issue tracking, pull requests, wikis, and a built-in CI runner. |
| **Listmonk** | [knadh/listmonk](https://github.com/knadh/listmonk) | 19.8k | Self-hosted newsletter and mailing-list manager. |
| **Mattermost** | [mattermost/mattermost](https://github.com/mattermost/mattermost) | 761.4k | Open-source team messaging and collaboration platform. |
| **Miniflux** | [miniflux/v2](https://github.com/miniflux/v2) | 76.1k | Minimalist self-hosted RSS and Atom feed reader. |
| **Pocketbase** | [pocketbase/pocketbase](https://github.com/pocketbase/pocketbase) | 122k | Open-source backend for web and mobile apps, shipped as one executable that bundles a database, user authentication, file storage, and an admin dashboard. |

Go LOC counts are taken at the pinned commit SHA, across all `.go` files
outside `vendor/` directories (tests included).

**Pinning discipline.** Each entry in the manifest pins a specific
commit SHA. The snippet tooling fetches upstream files at that SHA and
vendors them under `docs/site/snippets/external/<project>/` with a
provenance header, so every excerpt on this site is traceable back to
an exact upstream revision. Bumping a SHA is a deliberate act, not an
automatic one.
