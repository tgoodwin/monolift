# Lift-region candidate manifest

Master index of all merged lift-region candidates from SPRINT-0034 Phase 2b.
Each entry has a global ID (`<project>/<M-n>`) used to track downstream research (activation-path traces, graph derivation).

**Source:** `projects/<project>/merged.md` (Phase 2b aggregation output).
**Evaluation codebases:** [`evaluation/`](../../../../evaluation/) (pinned SHAs in [`evaluation/MANIFEST.yaml`](../../../../evaluation/MANIFEST.yaml)).

**Counts:** 88 candidates total. 59 high/medium-high confidence, 29 medium/low.

---

## caddy (12 candidates)

Source tree: [`evaluation/caddy/`](../../../../evaluation/caddy/)

| ID | Name | Region root | Confidence | Provenance | Trace status |
|---|---|---|---|---|---|
| caddy/M-1 | Goldmark markdown render (`funcMarkdown`) | [`tplcontext.go:350`](../../../../evaluation/caddy/modules/caddyhttp/templates/tplcontext.go) | high | 3/3 | — |
| caddy/M-2 | Buffered template execution (`executeTemplate`) | [`templates.go:455`](../../../../evaluation/caddy/modules/caddyhttp/templates/templates.go) | medium-high | 3/3 | — |
| caddy/M-3 | Bcrypt/Argon2 password verify (`correctPassword`) | [`basicauth.go:165`](../../../../evaluation/caddy/modules/caddyhttp/caddyauth/basicauth.go) | high | 2/3 | — |
| caddy/M-4 | Internal CA cert issuance (`InternalIssuer.Issue`) | [`internalissuer.go:103`](../../../../evaluation/caddy/modules/caddytls/internalissuer.go) | medium | 2/3 | — |
| caddy/M-5 | HTTP response compression (`Encode.ServeHTTP`) | [`encode.go:154`](../../../../evaluation/caddy/modules/caddyhttp/encode/encode.go) | medium | 2/3 disputed | — |
| caddy/M-6 | Caddyfile config adaptation (`Adapter.Adapt`) | [`adapter.go:32`](../../../../evaluation/caddy/caddyconfig/caddyfile/adapter.go) | low-medium | 2/3 disputed | — |
| caddy/M-7 | Directory listing (`loadDirectoryContents`) | [`browse.go:119`](../../../../evaluation/caddy/modules/caddyhttp/fileserver/browse.go) | medium | 1/3 weak | — |
| caddy/M-8 | FastCGI transport (`Transport.RoundTrip`) | [`fastcgi.go:163`](../../../../evaluation/caddy/modules/caddyhttp/reverseproxy/fastcgi/fastcgi.go) | low-medium | 1/3 weak | — |
| caddy/M-9 | Template file include (`funcInclude`/`funcReadFile`) | [`tplcontext.go:112`](../../../../evaluation/caddy/modules/caddyhttp/templates/tplcontext.go) | low | 1/3 weak | — |
| caddy/M-10 | Log filter encoder (`FilterEncoder.EncodeEntry`) | [`filterencoder.go:449`](../../../../evaluation/caddy/modules/logging/filterencoder.go) | low | 1/3 weak | — |
| caddy/M-11 | Reverse-proxy health check (`doActiveHealthCheck`) | [`healthchecks.go:391`](../../../../evaluation/caddy/modules/caddyhttp/reverseproxy/healthchecks.go) | low | 1/3 weak | — |
| caddy/M-12 | SRV upstream refresh (`SRVUpstreams.GetUpstreams`) | [`upstreams.go:122`](../../../../evaluation/caddy/modules/caddyhttp/reverseproxy/upstreams.go) | low | 1/3 weak | — |

---

## gitea (21 candidates)

Source tree: [`evaluation/gitea/`](../../../../evaluation/gitea/)

| ID | Name | Region root | Confidence | Provenance | Trace status |
|---|---|---|---|---|---|
| gitea/M-1 | Webhook delivery worker (`Deliver`) | [`deliver.go:153`](../../../../evaluation/gitea/services/webhook/deliver.go) | high | 3/3 | — |
| gitea/M-2 | Repository archive generator (`doArchive`) | [`archiver.go:146`](../../../../evaluation/gitea/services/repository/archiver/archiver.go) | high | 3/3 | — |
| gitea/M-3 | Avatar image processing (`ProcessAvatarImage`) | [`avatar.go:101`](../../../../evaluation/gitea/modules/avatar/avatar.go) | high | 2/3 | — |
| gitea/M-4 | Repository migration (`MigrateRepository`) | [`migrate.go:111`](../../../../evaluation/gitea/services/migrations/migrate.go) | high | 2/3 | — |
| gitea/M-5 | Code indexer (`index`) | [`indexer.go:41`](../../../../evaluation/gitea/modules/indexer/code/indexer.go) | high/medium | 2/3 | — |
| gitea/M-6 | Mirror pull sync (`runSync`) | [`mirror_pull.go:109`](../../../../evaluation/gitea/services/mirror/mirror_pull.go) | medium | 2/3 | — |
| gitea/M-7 | PR mergeability check (`checkPullRequestMergeable`) | [`check.go:427`](../../../../evaluation/gitea/services/pull/check.go) | high | 1/3 | — |
| gitea/M-8 | Renderable git diff (`GetDiffForRender`) | [`gitdiff.go:1333`](../../../../evaluation/gitea/services/gitdiff/gitdiff.go) | medium | 2/3 | — |
| gitea/M-9 | RPM repo metadata rebuild (`BuildSpecificRepositoryFiles`) | [`repository.go:163`](../../../../evaluation/gitea/services/packages/rpm/repository.go) | medium | 1/3 | — |
| gitea/M-10 | Push-update worker (`pushUpdates`) | [`push.go:77`](../../../../evaluation/gitea/services/repository/push.go) | medium | 1/3 | — |
| gitea/M-11 | Issue indexer handler | [`indexer.go:166`](../../../../evaluation/gitea/modules/indexer/issues/indexer.go) | medium | 1/3 | — |
| gitea/M-12 | Language statistics (`GetLanguageStats`) | [`language_stats_nogogit.go:22`](../../../../evaluation/gitea/modules/git/languagestats/language_stats_nogogit.go) | high/medium | 1/3 | — |
| gitea/M-13 | Mailer send (`sender.send`) | [`sender.go:17`](../../../../evaluation/gitea/services/mailer/sender/sender.go) | high | 1/3 | — |
| gitea/M-14 | Actions workflow detection (`DetectWorkflows`) | [`workflows.go:120`](../../../../evaluation/gitea/modules/actions/workflows.go) | medium | 1/3 | — |
| gitea/M-15 | Mirror LFS sync (`StoreMissingLfsObjectsInRepository`) | [`repo.go:61`](../../../../evaluation/gitea/modules/repository/repo.go) | medium | 1/3 | — |
| gitea/M-16 | Password hashing Argon2 (`Argon2Hasher.HashWithSaltBytes`) | [`argon2.go:29`](../../../../evaluation/gitea/modules/auth/password/hash/argon2.go) | medium-high | 1/3 | — |
| gitea/M-17 | Syntax highlighting (`RenderFullFile`) | [`highlight.go:124`](../../../../evaluation/gitea/modules/highlight/highlight.go) | medium | 1/3 | — |
| gitea/M-18 | Markdown render (`render`) | [`markdown.go:186`](../../../../evaluation/gitea/modules/markup/markdown/markdown.go) | low | 3/3 | — |
| gitea/M-19 | Debian repo metadata rebuild (`BuildSpecificRepositoryFiles`) | [`repository.go:154`](../../../../evaluation/gitea/services/packages/debian/repository.go) | high | OVERLOOKED | — |
| gitea/M-20 | PR merge-and-push (`doMergeAndPush`) | [`merge.go:334`](../../../../evaluation/gitea/services/pull/merge.go) | low | 1/3 disputed | — |
| gitea/M-21 | NPM package upload parsing (`ParsePackage`) | [`creator.go:203`](../../../../evaluation/gitea/modules/packages/npm/creator.go) | low | 1/3 disputed | — |

---

## listmonk (11 candidates)

Source tree: [`evaluation/listmonk/`](../../../../evaluation/listmonk/)

| ID | Name | Region root | Confidence | Provenance | Trace status |
|---|---|---|---|---|---|
| listmonk/M-1 | Per-recipient campaign message render | [`message.go:13`](../../../../evaluation/listmonk/internal/manager/message.go) | high | 3/3 | — |
| listmonk/M-2 | HTTP webhook delivery (`Postback.Push`) | [`postback.go:97`](../../../../evaluation/listmonk/internal/messenger/postback/postback.go) | high | 3/3 | — |
| listmonk/M-3 | SMTP message send (`Emailer.Push`) | [`email.go:111`](../../../../evaluation/listmonk/internal/messenger/email/email.go) | high | 3/3 | — |
| listmonk/M-4 | Image thumbnail generation (`processImage`) | [`media.go:212`](../../../../evaluation/listmonk/cmd/media.go) | high | 3/3 | — |
| listmonk/M-5 | Bulk subscriber CSV ingest (`Session.LoadCSV`) | [`importer.go:452`](../../../../evaluation/listmonk/internal/subimporter/importer.go) | high | 3/3 | — |
| listmonk/M-6 | SES/SNS bounce processing (`SES.ProcessBounce`) | [`ses.go:108`](../../../../evaluation/listmonk/internal/bounce/webhooks/ses.go) | high | 2/3 | — |
| listmonk/M-7 | Campaign template compilation (`CompileTemplate`) | [`campaigns.go:138`](../../../../evaluation/listmonk/models/campaigns.go) | medium | 2/3 | — |
| listmonk/M-8 | POP3 bounce mailbox scan (`POP.Scan`) | [`pop.go:79`](../../../../evaluation/listmonk/internal/bounce/mailbox/pop.go) | medium | 2/3 | — |
| listmonk/M-9 | Transactional message render (`TxMessage.Render`) | [`messages.go:74`](../../../../evaluation/listmonk/models/messages.go) | medium | 1/3 | — |
| listmonk/M-10 | Sendgrid bounce processing (`Sendgrid.ProcessBounce`) | [`sendgrid.go:53`](../../../../evaluation/listmonk/internal/bounce/webhooks/sendgrid.go) | medium | 1/3 | — |
| listmonk/M-11 | Campaign archive page render (`CampaignArchivePage`) | [`archive.go:119`](../../../../evaluation/listmonk/cmd/archive.go) | low-medium | 1/3 | — |

---

## mattermost (17 candidates)

Source tree: [`evaluation/mattermost/`](../../../../evaluation/mattermost/)

| ID | Name | Region root | Confidence | Provenance | Trace status |
|---|---|---|---|---|---|
| mattermost/M-1 | Document text extraction (`Extract`) | [`docextractor.go:21`](../../../../evaluation/mattermost/server/platform/services/docextractor/docextractor.go) | high | 3/3 | — |
| mattermost/M-2 | Image upload post-processing (`postprocessImage`) | [`file.go:931`](../../../../evaluation/mattermost/server/channels/app/file.go) | high | 3/3 | — |
| mattermost/M-3 | Outgoing webhook fan-out (`TriggerWebhook`) | [`webhook.go:99`](../../../../evaluation/mattermost/server/channels/app/webhook.go) | high | 2/3 | — |
| mattermost/M-4 | Elasticsearch bulk indexing (`BulkIndexPosts`) | [`indexing_job.go:412`](../../../../evaluation/mattermost/server/enterprise/elasticsearch/common/indexing_job.go) | high | 1/3 | — |
| mattermost/M-5 | Bulk team export (`BulkExport`) | [`export.go:113`](../../../../evaluation/mattermost/server/channels/app/export.go) | medium | 2/3 | — |
| mattermost/M-6 | Link-preview metadata fetch+parse (`getLinkMetadataForURL`) | [`post_metadata.go:1021`](../../../../evaluation/mattermost/server/channels/app/post_metadata.go) | high | 2/3 | — |
| mattermost/M-7 | Slash command HTTP execution (`DoCommandRequest`) | [`command.go:521`](../../../../evaluation/mattermost/server/channels/app/command.go) | medium | 1/3 | — |
| mattermost/M-8 | Push notification fan-out (`sendPushNotificationToAllSessions`) | [`notification_push.go:93`](../../../../evaluation/mattermost/server/channels/app/notification_push.go) | medium | 1/3 | — |
| mattermost/M-9 | Recap channel processing (`ProcessRecapChannel`) | [`recap.go:185`](../../../../evaluation/mattermost/server/channels/app/recap.go) | high | 1/3 | — |
| mattermost/M-10 | Remote-cluster file transfer (`sendFileToRemote`) | [`sendfile.go:84`](../../../../evaluation/mattermost/server/platform/services/remotecluster/sendfile.go) | high | 1/3 | — |
| mattermost/M-11 | Bulk import processing (`bulkImport`) | [`import.go:226`](../../../../evaluation/mattermost/server/channels/app/import.go) | medium | 1/3 | — |
| mattermost/M-12 | Per-recipient email render+send (`sendNotificationEmail`) | [`notification_email.go:144`](../../../../evaluation/mattermost/server/channels/app/notification_email.go) | medium | 1/3 | — |
| mattermost/M-13 | Batched email render+send (`sendBatchedEmailNotification`) | [`email_batching.go:252`](../../../../evaluation/mattermost/server/channels/app/email/email_batching.go) | medium-high | 1/3 | — |
| mattermost/M-14 | PBKDF2 password hashing (`PBKDF2.Hash`) | [`pbkdf2.go:151`](../../../../evaluation/mattermost/server/channels/app/password/hashers/pbkdf2.go) | high | 1/3 | — |
| mattermost/M-15 | Slack workspace import (`SlackImport`) | [`slackimport.go:131`](../../../../evaluation/mattermost/server/platform/services/slackimport/slackimport.go) | medium | 1/3 | — |
| mattermost/M-16 | File search (`SearchFilesInTeamForUser`) | [`file.go:1445`](../../../../evaluation/mattermost/server/channels/app/file.go) | low | 1/3 weak | — |
| mattermost/M-17 | LDAP user/group sync (`SyncLdap`) | [`ldap.go:18`](../../../../evaluation/mattermost/server/channels/app/ldap.go) | low | OVERLOOKED | — |

---

## miniflux (15 candidates)

Source tree: [`evaluation/miniflux/`](../../../../evaluation/miniflux/)

| ID | Name | Region root | Confidence | Provenance | Trace status |
|---|---|---|---|---|---|
| miniflux/M-1 | Full feed refresh (`RefreshFeed`) | [`handler.go:207`](../../../../evaluation/miniflux/internal/reader/handler/handler.go) | high | 3/3 | — |
| miniflux/M-2 | Per-entry scrape/sanitize loop (`ProcessFeedEntries`) | [`processor.go:27`](../../../../evaluation/miniflux/internal/reader/processor/processor.go) | high | 3/3 | — |
| miniflux/M-3 | HTML sanitizer (`SanitizeHTML`) | [`sanitizer.go:217`](../../../../evaluation/miniflux/internal/reader/sanitizer/sanitizer.go) | high | 3/3 | — |
| miniflux/M-4 | Integration push fan-out (`PushEntries`) | [`integration.go:511`](../../../../evaluation/miniflux/internal/integration/integration.go) | high | 3/3 | — |
| miniflux/M-5 | Feed icon discovery+resize (`UpdateOrCreateFeedIcon`) | [`checker.go:28`](../../../../evaluation/miniflux/internal/reader/icon/checker.go) | medium | 3/3 | — |
| miniflux/M-6 | Feed format parser (`ParseFeed`) | [`parser.go:20`](../../../../evaluation/miniflux/internal/reader/parser/parser.go) | high | 2/3 | — |
| miniflux/M-7 | Website scraper (`ScrapeWebsite`) | [`scraper.go:21`](../../../../evaluation/miniflux/internal/reader/scraper/scraper.go) | high | 2/3 | — |
| miniflux/M-8 | Readability extractor (`ExtractContent`) | [`readability.go:73`](../../../../evaluation/miniflux/internal/reader/readability/readability.go) | high | 2/3 | — |
| miniflux/M-9 | Per-entry save fan-out (`SendEntry`) | [`integration.go:41`](../../../../evaluation/miniflux/internal/integration/integration.go) | high | 2/3 | — |
| miniflux/M-10 | Feed subscription finder (`FindSubscriptions`) | [`finder.go:44`](../../../../evaluation/miniflux/internal/reader/subscription/finder.go) | medium | 2/3 | — |
| miniflux/M-11 | Media-proxy HTML rewrite (`RewriteDocumentWithAbsoluteProxyURL`) | [`rewriter.go:23`](../../../../evaluation/miniflux/internal/mediaproxy/rewriter.go) | medium-low | 2/3 disputed | — |
| miniflux/M-12 | OPML bulk import (`Import`) | [`handler.go:41`](../../../../evaluation/miniflux/internal/reader/opml/handler.go) | medium-low | 2/3 | — |
| miniflux/M-13 | User-pull full article fetch (`ProcessEntryWebPage`) | [`processor.go:180`](../../../../evaluation/miniflux/internal/reader/processor/processor.go) | medium | 1/3 weak | — |
| miniflux/M-14 | OAuth2 token exchange+profile (`googleProvider.Profile`) | [`google.go:57`](../../../../evaluation/miniflux/internal/oauth2/google.go) | medium | 1/3 weak | — |
| miniflux/M-15 | First-time feed create (`CreateFeed`) | [`handler.go:116`](../../../../evaluation/miniflux/internal/reader/handler/handler.go) | low | 1/3 disputed | — |

---

## pocketbase (12 candidates)

Source tree: [`evaluation/pocketbase/`](../../../../evaluation/pocketbase/)

| ID | Name | Region root | Confidence | Provenance | Trace status |
|---|---|---|---|---|---|
| pocketbase/M-1 | Image thumbnail generation (`CreateThumb`) | [`filesystem.go:489`](../../../../evaluation/pocketbase/tools/filesystem/filesystem.go) | high | 3/3 | — |
| pocketbase/M-2 | OAuth2 outbound exchange (`recordAuthWithOAuth2`) | [`record_auth_with_oauth2.go:30`](../../../../evaluation/pocketbase/apis/record_auth_with_oauth2.go) | medium | 3/3 | — |
| pocketbase/M-3 | Bcrypt password verify (`ValidatePassword`) | [`field_password.go:317`](../../../../evaluation/pocketbase/core/field_password.go) | high | 2/3 | — |
| pocketbase/M-4 | Backup archive zip writer (`archive.Create`) | [`create.go:18`](../../../../evaluation/pocketbase/tools/archive/create.go) | medium | 2/3 | — |
| pocketbase/M-5 | Password-reset mailer (`SendRecordPasswordReset`) | [`record.go:128`](../../../../evaluation/pocketbase/mails/record.go) | medium-high | 2/3 | — |
| pocketbase/M-6 | Bcrypt password hash on save (`setValue`) | [`field_password.go:286`](../../../../evaluation/pocketbase/core/field_password.go) | high | 1/3 | — |
| pocketbase/M-7 | SMTP send (`SMTPClient.send`) | [`smtp.go:62`](../../../../evaluation/pocketbase/tools/mailer/smtp.go) | high | 1/3 | — |
| pocketbase/M-8 | S3 multipart upload (`Uploader.Upload`) | [`uploader.go:71`](../../../../evaluation/pocketbase/tools/filesystem/internal/s3blob/s3/uploader.go) | high | 1/3 | — |
| pocketbase/M-9 | OAuth2 avatar download (`safeFileFromURL`) | [`record_auth_with_oauth2.go:468`](../../../../evaluation/pocketbase/apis/record_auth_with_oauth2.go) | medium | 1/3 | — |
| pocketbase/M-10 | Record relation expansion (`ExpandRecords`) | [`record_query_expand.go:34`](../../../../evaluation/pocketbase/core/record_query_expand.go) | medium | 2/3 | — |
| pocketbase/M-11 | Email template resolution (`resolveEmailTemplate`) | [`record.go:251`](../../../../evaluation/pocketbase/mails/record.go) | medium | 1/3 | — |
| pocketbase/M-12 | JavaScript hook execution (JSVM executor pool) | [`binds.go:81`](../../../../evaluation/pocketbase/plugins/jsvm/binds.go) | low | 1/3 | — |

---

## Summary by confidence tier

| Tier | Count | Projects |
|---|---|---|
| high / high-medium | 42 | all 6 |
| medium / medium-high | 29 | all 6 |
| low-medium / low | 17 | all 6 |

## Filtering for activation-path trace phase

For the pilot (caddy + miniflux), candidates at **medium confidence or above**:
- caddy: M-1 through M-5, M-7 (6 candidates)
- miniflux: M-1 through M-10, M-13, M-14 (12 candidates)

Total pilot set: **18 candidates**.
