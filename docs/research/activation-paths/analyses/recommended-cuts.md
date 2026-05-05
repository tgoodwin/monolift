# Recommended Cuts

Master recommendation table for SPRINT-0039. `mattermost/M-4` is retained as the structural gap.

| Trace ID | Path length | Recommended step | Recommended function | Edge type | Boundary data | State | Callbacks | Feasibility |
|---|---|---|---|---|---|---|---|---|
| `caddy/M-1` | 13 | 13 | `(TemplateContext).funcMarkdown` | `reflective-call-via-string-keyed-map` | Trivial | Stateless | 0 (confirmed) | Feasible |
| `caddy/M-2` | 12 | 12 | `(*Templates).executeTemplate` | `method-call-on-concrete-type` | Proxy-required | Config-only | 0 (confirmed) | Feasible-with-proxy |
| `caddy/M-3` | 11 | 11 | `(HTTPBasicAuth).correctPassword` | `method-call-on-concrete-type` | Serializable | Config-only | 0 (confirmed) | Feasible |
| `caddy/M-4` | 10 | 10 | `(InternalIssuer).Issue` | `interface-method-dispatch` | Serializable | Config-only | 0 (confirmed) | Feasible |
| `caddy/M-5` | 10 | 10 | `(*Encode).ServeHTTP` | `interface-method-dispatch` | Proxy-required | Shared-state | 0 (estimated) | Feasible-with-proxy |
| `caddy/M-7` | 9 | 9 | `(*FileServer).loadDirectoryContents` | `method-call-on-concrete-type` | Proxy-required | Shared-state | 0 (confirmed) | Feasible-with-proxy |
| `gitea/M-1` | 13 | 12 | `handler` | `closure-capture-of-struct-field` | Trivial | Client-reconstructible | Low | Feasible |
| `gitea/M-10` | 15 | 14 | `handler` | `function-value-in-struct-field` | Serializable | Client-reconstructible | Low | Feasible |
| `gitea/M-11` | 8 | 7 | `InitIssueIndexer` | `direct-function-call` | Trivial | Client-reconstructible | 0 (estimated) | Feasible |
| `gitea/M-12` | 10 | 8 | `handler` | `function-value-in-struct-field` | Serializable | Client-reconstructible | Low | Feasible |
| `gitea/M-13` | 6 | 6 | `send` | `call-through-package-level-function-variable` | Trivial | Client-reconstructible | 0 (confirmed) | Feasible |
| `gitea/M-14` | 9 | 9 | `DetectWorkflows` | `direct-function-call` | Reconstructible | Client-reconstructible | 0 (confirmed) | Feasible |
| `gitea/M-15` | 12 | 8 | `queueHandler` | `function-value-in-struct-field` | Serializable | Client-reconstructible | Low | Feasible |
| `gitea/M-16` | 8 | 8 | `(*Argon2Hasher).HashWithSaltBytes` | `interface-method-dispatch` | Serializable | Stateless | 0 (confirmed) | Feasible |
| `gitea/M-17` | 12 | 12 | `RenderFullFile` | `direct-function-call` | Serializable | Config-only | 0 (confirmed) | Feasible |
| `gitea/M-19` | 7 | 6 | `UploadPackageFile` | `http-handler-registration` | Serializable | Client-reconstructible | Low | Feasible |
| `gitea/M-2` | 13 | 12 | `handler` | `function-value-in-struct-field` | Serializable | Client-reconstructible | Low | Feasible |
| `gitea/M-3` | 7 | 5 | `UpdateAvatar` | `http-handler-registration` | Serializable | Client-reconstructible | Low | Feasible |
| `gitea/M-4` | 7 | 5 | `Migrate` | `http-handler-registration` | Serializable | Client-reconstructible | Low | Feasible |
| `gitea/M-5` | 9 | 8 | `registered` | `function-value-in-struct-field` | Serializable | Client-reconstructible | 0 (estimated) | Feasible |
| `gitea/M-6` | 13 | 10 | `queueHandler` | `function-value-in-struct-field` | Serializable | Client-reconstructible | Low | Feasible |
| `gitea/M-7` | 6 | 6 | `checkPullRequestMergeable` | `direct-function-call` | Trivial | Client-reconstructible | 0 (confirmed) | Feasible |
| `gitea/M-8` | 7 | 7 | `GetDiffForRender` | `direct-function-call` | Reconstructible | Client-reconstructible | 0 (confirmed) | Feasible |
| `gitea/M-9` | 7 | 6 | `UploadPackageFile` | `http-handler-registration` | Serializable | Client-reconstructible | Low | Feasible |
| `listmonk/M-1` | 4 | 4 | `(*Manager).NewCampaignMessage` | `method-call-on-concrete-type` | Trivial | Client-reconstructible | 0 (confirmed) | Feasible |
| `listmonk/M-10` | 4 | 3 | `(*App).BounceWebhook` | `http-handler-registration` | Reconstructible | Client-reconstructible | Low | Feasible |
| `listmonk/M-2` | 4 | 4 | `(*Postback).Push` | `map-lookup-interface-method-dispatch` | Trivial | Client-reconstructible | 0 (confirmed) | Feasible |
| `listmonk/M-3` | 3 | 3 | `(*Emailer).Push` | `interface-method-dispatch` | Reconstructible | Config-only | 0 (confirmed) | Feasible |
| `listmonk/M-4` | 4 | 3 | `(*App).UploadMedia` | `callback-registration` (method-value + closure-wrapper)` | Reconstructible | Client-reconstructible | Low | Feasible |
| `listmonk/M-5` | 4 | 3 | `(*App).ImportSubscribers` | `http-handler-registration-via-wrapper-closure` | Reconstructible | Client-reconstructible | Low | Feasible |
| `listmonk/M-6` | 4 | 3 | `(*App).BounceWebhook` | `method-value-handler-registration` | Reconstructible | Client-reconstructible | Low | Feasible |
| `listmonk/M-7` | 4 | 4 | `(*Campaign).CompileTemplate` | `method-call-on-concrete-type` | Trivial | Config-only | 0 (confirmed) | Feasible |
| `listmonk/M-8` | 3 | 3 | `(*POP).Scan` | `interface-method-dispatch` | Proxy-required | Client-reconstructible | 0 (confirmed) | Feasible-with-proxy |
| `listmonk/M-9` | 5 | 3 | `anonymous` | `http-handler-registration` | Trivial | Shared-state | Low | Feasible |
| `mattermost/M-1` | 10 | 10 | `Extract` | `direct-function-call` | Trivial | Client-reconstructible | 0 (confirmed) | Feasible |
| `mattermost/M-10` | 9 | 9 | `(*Service).sendFileToRemote` | `method-call-on-concrete-type` | Serializable | Client-reconstructible | 0 (confirmed) | Feasible |
| `mattermost/M-11` | 5 | 3 | `bulkImportCmdF` | `function-value-in-struct-field` | Serializable | Client-reconstructible | Low | Feasible |
| `mattermost/M-12` | 12 | 12 | `(*App).sendNotificationEmail` | `method-call-on-concrete-type` | Reconstructible | Shared-state | 0 (confirmed) | Feasible |
| `mattermost/M-13` | 11 | 11 | `(*Service).sendBatchedEmailNotification` | `indirect-call-through-parameter` | Serializable | Client-reconstructible | 0 (confirmed) | Feasible |
| `mattermost/M-14` | 12 | 12 | `(PBKDF2).Hash` | `interface-method-dispatch` | Trivial | Stateless | 0 (confirmed) | Feasible |
| `mattermost/M-15` | 4 | 2 | `slackImportCmdF` | `function-value-in-struct-field` | Serializable | Shared-state | Low | Feasible |
| `mattermost/M-2` | 9 | 7 | `uploadFileSimple` | `direct-function-call` | Serializable | Client-reconstructible | 0 (estimated) | Feasible |
| `mattermost/M-3` | 14 | 13 | `(*App).handleWebhookEvents` | `method-call-on-concrete-type` | Reconstructible | Shared-state | Low | Feasible |
| `mattermost/M-4` | 11 | gap | `target-not-found` | `target-not-found` | - | - | - | Infeasible |
| `mattermost/M-5` | 4 | 3 | `bulkExportCmdF` | `function-value-in-struct-field` | Serializable | Client-reconstructible | 0 (estimated) | Feasible |
| `mattermost/M-6` | 13 | 13 | `(*App).getLinkMetadataForURL` | `method-call-on-concrete-type` | Reconstructible | Shared-state | 0 (confirmed) | Feasible |
| `mattermost/M-7` | 13 | 13 | `(*App).DoCommandRequest` | `method-call-on-concrete-type` | Reconstructible | Shared-state | 0 (confirmed) | Feasible |
| `mattermost/M-8` | 9 | 9 | `(*App).sendPushNotificationToAllSessions` | `method-call-on-concrete-type` | Reconstructible | Shared-state | 0 (confirmed) | Feasible |
| `mattermost/M-9` | 13 | 12 | `execute` | `function-value-in-struct-field` | Reconstructible | Shared-state | 0 (estimated) | Feasible |
| `miniflux/M-1` | 5 | 5 | `RefreshFeed` | `direct-function-call` | Reconstructible | Client-reconstructible | 0 (confirmed) | Feasible |
| `miniflux/M-10` | 7 | 7 | `(*subscriptionFinder).FindSubscriptions` | `method-call-on-concrete-type` | Trivial | Client-reconstructible | 0 (confirmed) | Feasible |
| `miniflux/M-13` | 7 | 7 | `ProcessEntryWebPage` | `direct-function-call` | Serializable | Client-reconstructible | 0 (confirmed) | Feasible |
| `miniflux/M-14` | 7 | 7 | `(*googleProvider).Profile` | `interface-method-dispatch` | Serializable | Client-reconstructible | 0 (confirmed) | Feasible |
| `miniflux/M-2` | 5 | 5 | `ProcessFeedEntries` | `direct-function-call` | Reconstructible | Client-reconstructible | 0 (confirmed) | Feasible |
| `miniflux/M-3` | 6 | 6 | `SanitizeHTML` | `direct-function-call` | Trivial | Stateless | 0 (confirmed) | Feasible |
| `miniflux/M-4` | 5 | 5 | `PushEntries` | `goroutine-launch-of-named-function` | Serializable | Client-reconstructible | 0 (confirmed) | Feasible |
| `miniflux/M-5` | 7 | 7 | `(*iconChecker).UpdateOrCreateFeedIcon` | `method-call-on-concrete-type` | Serializable | Client-reconstructible | 0 (confirmed) | Feasible |
| `miniflux/M-6` | 5 | 5 | `ParseFeed` | `direct-function-call` | Serializable | Stateless | 0 (confirmed) | Feasible |
| `miniflux/M-7` | 6 | 6 | `ScrapeWebsite` | `direct-function-call` | Trivial | Client-reconstructible | 0 (confirmed) | Feasible |
| `miniflux/M-8` | 7 | 6 | `ScrapeWebsite` | `direct-function-call` | Trivial | Client-reconstructible | 0 (estimated) | Feasible |
| `miniflux/M-9` | 8 | 8 | `SendEntry` | `goroutine-launch` | Serializable | Client-reconstructible | 0 (confirmed) | Feasible |
| `pocketbase/M-1` | 11 | 11 | `(*System).CreateThumb` | `method-call-on-concrete-type` | Reconstructible | Client-reconstructible | 0 (confirmed) | Feasible |
| `pocketbase/M-10` | 9 | 9 | `(*BaseApp).ExpandRecords` | `interface-method-dispatch` | Reconstructible | Client-reconstructible | 0 (confirmed) | Feasible |
| `pocketbase/M-11` | 9 | 9 | `resolveEmailTemplate` | `direct-function-call` | Serializable | Config-only | 0 (confirmed) | Feasible |
| `pocketbase/M-2` | 8 | 8 | `recordAuthWithOAuth2` | `function-value-as-argument` | Trivial | Config-only | 0 (confirmed) | Feasible |
| `pocketbase/M-3` | 8 | 8 | `PasswordFieldValue.Validate` | `type-asserted-method-call` | Trivial | Stateless | 0 (confirmed) | Feasible |
| `pocketbase/M-4` | 10 | 9 | `Create` | `direct-function-call` | Trivial | Client-reconstructible | 0 (estimated) | Feasible |
| `pocketbase/M-5` | 9 | 9 | `SendRecordPasswordReset` | `direct-function-call` | Trivial | Config-only | 0 (confirmed) | Feasible |
| `pocketbase/M-6` | 8 | 8 | `(*PasswordField).setValue` | `method-value-call` | Serializable | Stateless | 0 (confirmed) | Feasible |
| `pocketbase/M-7` | 12 | 12 | `(*SMTPClient).send` | `method-call-on-concrete-type` | Reconstructible | Config-only | 0 (confirmed) | Feasible |
| `pocketbase/M-8` | 11 | 9 | `(*writer).Write` | `interface-method-dispatch` | Serializable | Client-reconstructible | Low | Feasible |
| `pocketbase/M-9` | 16 | 16 | `safeFileFromURL` | `direct-function-call` | Reconstructible | Config-only | 0 (confirmed) | Feasible |
