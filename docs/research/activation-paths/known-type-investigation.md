# Known-type investigation: 18 traces with false Infeasible classification

The boundary-data classifier (`classifyBoundaryData` in `pkg/activation/cut_boundary.go`)
gates candidates as Infeasible when their parameter or return types contain sync primitives,
function values, or unrecognized streaming types. For 18 corpus traces, this gating is
too aggressive: types that a human analyst classified as Reconstructible or Serializable
are being marked Infeasible, causing the "no competing feasible candidate remained" diagnostic.

This investigation identifies the specific type causing each false classification.

## Root cause patterns

Four patterns account for all 18 traces:

### Pattern A: Framework context interface with function-typed methods

Framework request-context interfaces (`echo.Context`, gitea's `context.ResponseWriter`,
mattermost's `request.CTX`) include methods that accept or return function types (e.g.
`Handler() HandlerFunc`, `SetHandler(HandlerFunc)`, `GetT() TranslateFunc`,
`With(func(CTX) CTX)`, `Before(func(ResponseWriter))`). The interface method walker
(`classifyBoundaryInterface` -> `classifyBoundaryMethodSignature`) encounters these
function-typed parameters/returns, hits the `*types.Signature` case, and returns
`BoundaryInfeasible`. This propagates up to the overall interface classification.

These interfaces are actually Reconstructible: they wrap an HTTP request/response
pair and can be reconstituted from serialized request data on the remote side.

**Affected traces:** listmonk/M-4, M-5, M-6, M-9, M-10; gitea/M-3; mattermost/M-8

### Pattern B: Struct containing `sync.Mutex` / `sync.RWMutex` in unexported fields

Domain structs carry mutex fields for internal thread safety (`git.Repository.mu sync.Mutex`,
`store.Store.mu sync.RWMutex` reachable through `core.Record`). The struct field walker
(`classifyBoundaryStruct`) walks ALL fields including unexported ones, hits the
`knownBoundaryType` match for `sync.Mutex`/`sync.RWMutex`, and returns `BoundaryInfeasible`.

These structs are actually Serializable or Reconstructible: the mutex protects runtime
concurrency and is irrelevant to the serialized representation. A fresh mutex can be
initialized on the remote side.

**Affected traces:** pocketbase/M-2, M-6, M-8 (path peers); gitea/M-8, M-14; gitea/M-11, M-17 (path peers)

### Pattern C: Interface with function-typed method parameters (distinct from Pattern A)

Non-HTTP interfaces contain methods that accept function callbacks:
`core.App.RunInTransaction(fn func(txApp App) error)`,
`request.CTX.GetT() i18n.TranslateFunc` (returns a function type),
`request.CTX.With(func(ctx CTX) CTX)`.

The classifier sees the `func(...)` signature in the method's parameter or return
list and marks the entire interface as `BoundaryInfeasible`.

These interfaces represent application-level service abstractions. The function-typed
methods are for transactional/callback patterns that don't need to cross the network --
the remote side would use a local implementation or RPC stub.

**Affected traces:** pocketbase/M-5, M-10, M-11; mattermost/M-8

### Pattern D: Struct with function-typed exported fields

`cobra.Command` has ~15 exported fields of function type (`Run`, `PreRun`, `PostRun`,
`Args`, `ValidArgsFunction`, etc.). The struct walker finds these `*types.Signature`
fields and classifies the struct as `BoundaryInfeasible`.

`cobra.Command` is actually Serializable for boundary purposes: only the command's
parsed flags and positional arguments need to cross the network, not the lifecycle
callbacks.

**Affected traces:** mattermost/M-5, M-11, M-15

## Per-trace detail

| Trace ID | Expected function | Problematic type | Why classifier marks Infeasible | Current class | Human class | Pattern |
|---|---|---|---|---|---|---|
| listmonk/M-4 | `(*App).UploadMedia` | `echo.Context` (param) | Interface method `Handler() HandlerFunc` returns `func(Context) error` | BoundaryInfeasible | Reconstructible | A |
| listmonk/M-5 | `(*App).ImportSubscribers` | `echo.Context` (param) | Interface method `Handler() HandlerFunc` returns `func(Context) error` | BoundaryInfeasible | Reconstructible | A |
| listmonk/M-6 | `(*App).BounceWebhook` | `echo.Context` (param) | Interface method `Handler() HandlerFunc` returns `func(Context) error` | BoundaryInfeasible | Reconstructible | A |
| listmonk/M-9 | `anonymous` (echo handler) | `echo.Context` (param) | Interface method `Handler() HandlerFunc` returns `func(Context) error` | BoundaryInfeasible | Trivial | A |
| listmonk/M-10 | `(*App).BounceWebhook` | `echo.Context` (param) | Interface method `Handler() HandlerFunc` returns `func(Context) error` | BoundaryInfeasible | Reconstructible | A |
| pocketbase/M-2 | `recordAuthWithOAuth2` | `*core.RequestEvent` (param) | Struct field `mu sync.Mutex`; also `App` field has func-typed methods | BoundaryInfeasible | Trivial | B+C |
| pocketbase/M-5 | `SendRecordPasswordReset` | `core.App` (param) | Interface method `RunInTransaction(fn func(txApp App) error)` has func param | BoundaryInfeasible | Trivial | C |
| pocketbase/M-6 | `(*PasswordField).setValue` | `*core.Record` (param) | `Record.data` is `*store.Store` which has `mu sync.RWMutex` | BoundaryInfeasible | Serializable | B |
| pocketbase/M-8 | `(*writer).Write` | path peers: `core.App`, `*core.RequestEvent` | `(*writer).Write([]byte)(int,error)` itself is Serializable; other project candidates gated by `core.App`/`*RequestEvent` | Serializable (self), peers gated | Serializable | B+C (peers) |
| pocketbase/M-10 | `(*BaseApp).ExpandRecords` | `ExpandFetchFunc` (param) | `ExpandFetchFunc` is `func(*Collection, []string) ([]*Record, error)` -- function type | BoundaryInfeasible | Reconstructible | C |
| pocketbase/M-11 | `resolveEmailTemplate` | `core.App` (param) | Interface method `RunInTransaction(fn func(txApp App) error)` has func param | BoundaryInfeasible | Serializable | C |
| gitea/M-3 | `UpdateAvatar` | `*context.APIContext` (param) | Embedded `Base.Resp` is `ResponseWriter` interface with `Before(fn func(ResponseWriter))` | BoundaryInfeasible | Serializable | A |
| gitea/M-8 | `GetDiffForRender` | `*git.Repository` (param) | Struct field `mu sync.Mutex` | BoundaryInfeasible | Reconstructible | B |
| gitea/M-11 | `InitIssueIndexer` | path peers: `*context.APIContext`, `*git.Repository` | `InitIssueIndexer(bool)` itself is Trivial; other project candidates gated by `*APIContext` (pattern A) | Trivial (self), peers gated | Trivial | A (peers) |
| gitea/M-14 | `DetectWorkflows` | `*git.Repository` (param), `*git.Commit` (param) | `Repository.mu sync.Mutex`; `Commit` embeds `Tree` which has `repo *Repository` | BoundaryInfeasible | Reconstructible | B |
| gitea/M-17 | `RenderFullFile` | path peers: `*context.APIContext`, `*git.Repository` | `RenderFullFile(string, string, []byte)` itself is Serializable; other project candidates gated | Serializable (self), peers gated | Serializable | A+B (peers) |
| mattermost/M-5 | `bulkExportCmdF` | `*cobra.Command` (param) | Struct fields `Run`, `PreRun`, `Args`, `ValidArgsFunction`, etc. are function types | BoundaryInfeasible | Serializable | D |
| mattermost/M-8 | `sendPushNotificationToAllSessions` | `request.CTX` (param) | Interface methods `GetT() TranslateFunc` returns func type; `With(func(CTX) CTX)` takes func param | BoundaryInfeasible | Reconstructible | A+C |
| mattermost/M-11 | `bulkImportCmdF` | `*cobra.Command` (param) | Struct fields `Run`, `PreRun`, `Args`, `ValidArgsFunction`, etc. are function types | BoundaryInfeasible | Serializable | D |
| mattermost/M-15 | `slackImportCmdF` | `*cobra.Command` (param) | Struct fields `Run`, `PreRun`, `Args`, `ValidArgsFunction`, etc. are function types | BoundaryInfeasible | Serializable | D |

## Suggested fixes (by pattern)

| Pattern | Root cause | Count | Suggested fix |
|---|---|---|---|
| A | Framework context interface has func-typed accessor methods | 7 direct + 2 peer | Add `echo.Context`, `request.CTX`, and gitea's `context.ResponseWriter` to `knownBoundaryType()` as Reconstructible. Alternatively, teach `classifyBoundaryInterface` to skip methods whose return type is a named function type (treat as "accessor for a callback registry" rather than "this interface IS a function"). |
| B | Struct with unexported `sync.Mutex`/`sync.RWMutex` field | 5 direct + 2 peer | Skip `sync.Mutex`/`sync.RWMutex` fields during struct classification (they protect runtime concurrency, not serializable state). A fresh zero-value mutex is always valid. Alternatively, downgrade mutex fields from `BoundaryInfeasible` to `Reconstructible` with reason "mutex can be zero-initialized on remote side". |
| C | Interface has methods with function-typed params/returns | 5 direct + 2 peer | Add `core.App` to `knownBoundaryType()` as Reconstructible. More generally, consider a heuristic: if an interface has N methods and only 1-2 involve function types (transaction wrappers, callback registries), treat the interface as Serializable rather than letting the func-typed minority dominate. |
| D | `*cobra.Command` struct has ~15 function-typed fields | 3 | Add `cobra.Command` to `knownBoundaryType()` as Serializable. For CLI dispatch, only flags and args cross the boundary; the lifecycle callbacks are irrelevant. |

## Priority

Pattern B (sync.Mutex in struct fields) is the highest-leverage fix: it is a general
rule change (not a per-type override) and would resolve 5 direct cases plus unblock
2 peer-gated cases. Pattern A (framework context override) is next, resolving 7 direct
cases. Together, these two fixes would address 14 of the 18 traces.
