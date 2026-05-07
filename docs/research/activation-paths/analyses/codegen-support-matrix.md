# SPRINT-0041 Codegen Support Matrix

This matrix is derived from [`recommended-cuts.md`](recommended-cuts.md). The
current table has 72 rows: 71 corpus traces plus the retained
`mattermost/M-4` structural gap. The sprint shorthand "62/71" maps to the
non-`Shared-state` runway. The 10 `Shared-state` rows are excluded from phase-1
HTTP/JSON generation:

`caddy/M-5`, `caddy/M-7`, `listmonk/M-9`, `mattermost/M-12`,
`mattermost/M-15`, `mattermost/M-3`, `mattermost/M-6`, `mattermost/M-7`,
`mattermost/M-8`, `mattermost/M-9`.

Status values are intentionally conservative:

- **generator-eligible** means current SPRINT-0041 codegen has an exercised
  codec and reconstructor path.
- **needs-new-reconstructor** means boundary values are in the trivial,
  serializable, or reconstructible class, but some reconstructed state family is
  not yet covered by the registry.
- **needs-shape/codec-work** means the blocker is not primarily state
  reconstruction: proxy boundary, receiver/method stubs, streaming values, or
  unresolved target.

| Trace | Function | Boundary codec coverage | Reconstructor coverage | Status |
|---|---|---|---|---|
| `caddy/M-1` | `(TemplateContext).funcMarkdown` | yes: trivial | none required | needs-shape/codec-work: receiver/method stub support |
| `caddy/M-2` | `(*Templates).executeTemplate` | no: proxy-required boundary | config-only not modeled | needs-shape/codec-work: cut deeper per ADR-0028 |
| `caddy/M-3` | `(HTTPBasicAuth).correctPassword` | yes: serializable | config-only not modeled | needs-new-reconstructor: config snapshot/env family |
| `caddy/M-4` | `(InternalIssuer).Issue` | yes: serializable | config-only not modeled | needs-new-reconstructor: issuer/config family |
| `gitea/M-1` | `handler` | yes: trivial | not covered | needs-new-reconstructor: web context/DB state |
| `gitea/M-10` | `handler` | yes: serializable | not covered | needs-new-reconstructor: web context/DB state |
| `gitea/M-11` | `InitIssueIndexer` | yes: trivial | not covered | needs-new-reconstructor: indexer family |
| `gitea/M-12` | `handler` | yes: serializable | not covered | needs-new-reconstructor: web context/DB state |
| `gitea/M-13` | `send` | yes: trivial | not covered | needs-new-reconstructor: mailer/notification sender |
| `gitea/M-14` | `DetectWorkflows` | yes after state split | not covered | needs-new-reconstructor: repository/git context |
| `gitea/M-15` | `queueHandler` | yes: serializable | not covered | needs-new-reconstructor: queue/DB family |
| `gitea/M-16` | `(*Argon2Hasher).HashWithSaltBytes` | yes: serializable | none required | needs-shape/codec-work: receiver/method stub support |
| `gitea/M-17` | `RenderFullFile` | yes: serializable | config-only not modeled | needs-new-reconstructor: rendering/config family |
| `gitea/M-19` | `UploadPackageFile` | yes: serializable | not covered | needs-new-reconstructor: package storage/DB family |
| `gitea/M-2` | `handler` | yes: serializable | not covered | needs-new-reconstructor: web context/DB state |
| `gitea/M-3` | `UpdateAvatar` | yes: serializable | not covered | needs-new-reconstructor: object storage/DB family |
| `gitea/M-4` | `Migrate` | yes: serializable | not covered | needs-new-reconstructor: repository migration state |
| `gitea/M-5` | `registered` | yes: serializable | not covered | needs-new-reconstructor: registry/DB family |
| `gitea/M-6` | `queueHandler` | yes: serializable | not covered | needs-new-reconstructor: queue/DB family |
| `gitea/M-7` | `checkPullRequestMergeable` | yes: trivial | not covered | needs-new-reconstructor: repository/DB family |
| `gitea/M-8` | `GetDiffForRender` | yes after state split | not covered | needs-new-reconstructor: git repository context |
| `gitea/M-9` | `UploadPackageFile` | yes: serializable | not covered | needs-new-reconstructor: package storage/DB family |
| `listmonk/M-1` | `(*Manager).NewCampaignMessage` | yes: trivial | not covered | needs-new-reconstructor: campaign manager/DB family |
| `listmonk/M-10` | `(*App).BounceWebhook` | yes after state split | not covered | needs-new-reconstructor: app DB/config family |
| `listmonk/M-2` | `(*Postback).Push` | yes: trivial | not covered | needs-new-reconstructor: postback HTTP/client state |
| `listmonk/M-3` | `(*Emailer).Push` | yes after state split | config-only not modeled | needs-new-reconstructor: mailer/SMTP family |
| `listmonk/M-4` | `(*App).UploadMedia` | yes after state split | not covered | needs-new-reconstructor: media store/DB family |
| `listmonk/M-5` | `(*App).ImportSubscribers` | yes after state split | not covered | needs-new-reconstructor: app DB/import state |
| `listmonk/M-6` | `(*App).BounceWebhook` | yes after state split | not covered | needs-new-reconstructor: app DB/config family |
| `listmonk/M-7` | `(*Campaign).CompileTemplate` | yes: trivial | config-only not modeled | needs-new-reconstructor: template/config family |
| `listmonk/M-8` | `(*POP).Scan` | no: proxy-required boundary | not covered | needs-shape/codec-work: cut deeper per ADR-0028 |
| `mattermost/M-1` | `Extract` | yes: trivial | not covered | needs-new-reconstructor: logger/settings/extractor state |
| `mattermost/M-10` | `(*Service).sendFileToRemote` | yes: serializable | not covered | needs-new-reconstructor: remote file/client state |
| `mattermost/M-11` | `bulkImportCmdF` | yes: serializable | not covered | needs-new-reconstructor: command/app state |
| `mattermost/M-13` | `(*Service).sendBatchedEmailNotification` | yes: serializable | not covered | needs-new-reconstructor: mailer/template state |
| `mattermost/M-14` | `(PBKDF2).Hash` | yes: trivial | none required | needs-shape/codec-work: receiver/method stub support |
| `mattermost/M-2` | `uploadFileSimple` | yes: serializable | not covered | needs-new-reconstructor: file store/app state |
| `mattermost/M-4` | `target-not-found` | no target | no target | needs-shape/codec-work: structural activation gap |
| `mattermost/M-5` | `bulkExportCmdF` | yes: serializable | not covered | needs-new-reconstructor: command/app state |
| `miniflux/M-1` | `RefreshFeed` | yes after state split | yes: `*storage.Storage` via SQL wrapper | generator-eligible |
| `miniflux/M-10` | `(*subscriptionFinder).FindSubscriptions` | yes: trivial | not covered | needs-new-reconstructor: finder/fetcher state |
| `miniflux/M-13` | `ProcessEntryWebPage` | yes: serializable | not covered | needs-new-reconstructor: feed/user/storage context |
| `miniflux/M-14` | `(*googleProvider).Profile` | yes: serializable | not covered | needs-new-reconstructor: OIDC/provider HTTP state |
| `miniflux/M-2` | `ProcessFeedEntries` | yes after state split | yes for storage only; other state not audited | needs-new-reconstructor: processor dependencies |
| `miniflux/M-3` | `SanitizeHTML` | yes: primitive/JSON params | none required | generator-eligible |
| `miniflux/M-4` | `PushEntries` | yes: serializable | not covered | needs-new-reconstructor: integrations config/client state |
| `miniflux/M-5` | `(*iconChecker).UpdateOrCreateFeedIcon` | yes: serializable | not covered | needs-new-reconstructor: icon checker/storage state |
| `miniflux/M-6` | `ParseFeed` | partial: `io.ReadSeeker` needs byte codec | none required | needs-shape/codec-work: streaming-to-`[]byte` codec |
| `miniflux/M-7` | `ScrapeWebsite` | yes: trivial | not covered | needs-new-reconstructor: fetch request builder/client |
| `miniflux/M-8` | `ScrapeWebsite` | yes: trivial | not covered | needs-new-reconstructor: fetch request builder/client |
| `miniflux/M-9` | `SendEntry` | yes: serializable | not covered | needs-new-reconstructor: integration clients |
| `pocketbase/M-1` | `(*System).CreateThumb` | yes after state split | not covered | needs-new-reconstructor: filesystem/image state |
| `pocketbase/M-10` | `(*BaseApp).ExpandRecords` | yes after state split | not covered | needs-new-reconstructor: app/DAO state |
| `pocketbase/M-11` | `resolveEmailTemplate` | yes: serializable | config-only not modeled | needs-new-reconstructor: mail template/config state |
| `pocketbase/M-2` | `recordAuthWithOAuth2` | yes: trivial | config-only not modeled | needs-new-reconstructor: OAuth provider config |
| `pocketbase/M-3` | `PasswordFieldValue.Validate` | yes: trivial | none required | needs-shape/codec-work: receiver/value-method support |
| `pocketbase/M-4` | `Create` | yes: trivial | not covered | needs-new-reconstructor: filesystem/archive context |
| `pocketbase/M-5` | `SendRecordPasswordReset` | yes: trivial | config-only not modeled | needs-new-reconstructor: mailer/config state |
| `pocketbase/M-6` | `(*PasswordField).setValue` | yes: serializable | none required | needs-shape/codec-work: receiver/method stub support |
| `pocketbase/M-7` | `(*SMTPClient).send` | yes after state split | config-only not modeled | needs-new-reconstructor: SMTP client/config family |
| `pocketbase/M-8` | `(*writer).Write` | yes: serializable | not covered | needs-new-reconstructor: writer/filesystem state |
| `pocketbase/M-9` | `safeFileFromURL` | yes after state split | config-only not modeled | needs-new-reconstructor: HTTP/client policy config |

## Summary

Current SPRINT-0041 codegen is proven on 2 rows:

- `miniflux/M-3` (`SanitizeHTML`) for stateless HTTP/JSON.
- `miniflux/M-1` (`RefreshFeed`) for SQL-backed state reconstruction.

Most remaining non-shared rows already have a plausible boundary codec at the
recommended-cut level, but need type-specific reconstructor registry entries
before generation should be accepted. The largest gaps are DB/app handles,
HTTP/fetch clients, mailers, object/file stores, and indexers.

## Next Reconstructor Families

Each item below should become a registry entry keyed by Go type identity and
assembled by the generator from typed imports/init/close snippets. These should
not become bespoke templates for a particular corpus application.

| Family | Registry key shape | Startup inputs | Close hook | Corpus pressure |
|---|---|---|---|---|
| DB pools: postgres | `*sql.DB`, wrappers containing `*sql.DB`, app stores whose constructor takes `*sql.DB` | `DATABASE_URL`, optional max-open/max-idle/lifetime env vars | `db.Close()` | Miniflux `*storage.Storage`; many Gitea/Listmonk/PocketBase app handles |
| DB pools: mysql | `*sql.DB` plus mysql driver-backed store wrappers | `MYSQL_DSN` or `DATABASE_URL`, pool env vars | `db.Close()` | Gitea and Listmonk deployments that use mysql-compatible stores |
| DB pools: sqlite | `*sql.DB` plus sqlite driver-backed store wrappers | `SQLITE_PATH` or `DATABASE_URL`, WAL/busy-timeout env vars | `db.Close()` | PocketBase app/DAO state and local-file DB cases |
| HTTP clients | `*http.Client`, wrappers with one `*http.Client`, request-builder structs with client fields | timeout, proxy, TLS, allow-list env vars | none, unless wrapper exposes close | Miniflux fetcher/scraper, PocketBase OAuth/download, Mattermost remote file calls |
| Mailers | SMTP client structs, emailer interfaces with concrete SMTP implementations | host, port, username, password, TLS mode, sender env vars | optional `Close()` when supported | Gitea notifications, Listmonk emailer, Mattermost batched email, PocketBase password reset |
| Object stores | S3/GCS/Azure/minio client structs, filesystem store wrappers | endpoint, bucket, region, credentials, path prefix env vars | optional client close | Gitea package/avatar storage, Listmonk media upload, Mattermost file store, PocketBase archive/thumb paths |
| Loggers | `*log.Logger`, `*slog.Logger`, zap/logrus-style logger structs | log level, format, destination env vars | sync/flush when supported | Mattermost extractor/logger state, general service diagnostics |
| Indexers | search/indexer service structs and queue-backed index writers | index DSN/path, queue/topic name, batch settings env vars | close/flush when supported | Gitea issue/repo indexers and similar corpus indexing jobs |

The admission rule should remain simple: if a reconstructed parameter's exact
type or wrapper pattern has no registry entry, refuse generation with
`missing_reconstructor` and name the type. Adding a corpus family means adding a
registry matcher, constructor snippet, imports, close behavior, and tests.
