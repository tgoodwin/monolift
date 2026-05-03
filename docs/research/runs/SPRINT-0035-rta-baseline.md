# SPRINT-0035 RTA Baseline

- Traces: 72
- Reachable: 49 (68.1%)
- Mean Tier 2 exact: 0.173
- Mean Tier 2 fuzzy: 0.199
- Mean Tier 3 file:line: 0.000

## Per Project

| Project | Traces | Reachable | Reachability | Mean T2 Exact | Mean T2 Fuzzy | Mean T3 |
|---|---:|---:|---:|---:|---:|---:|
| caddy | 6 | 0 | 0.0% | 0.000 | 0.000 | 0.000 |
| gitea | 18 | 16 | 88.9% | 0.133 | 0.159 | 0.000 |
| listmonk | 10 | 10 | 100.0% | 0.565 | 0.576 | 0.000 |
| mattermost | 15 | 0 | 0.0% | 0.000 | 0.000 | 0.000 |
| miniflux | 12 | 12 | 100.0% | 0.356 | 0.451 | 0.000 |
| pocketbase | 11 | 11 | 100.0% | 0.015 | 0.028 | 0.000 |

## Feasibility

| Project | Pattern | Completed | Timed out | Wall ms | Heap bytes | Error |
|---|---|---:|---:|---:|---:|---|
| miniflux | `.` | true | false | 3241 | 1655259896 |  |
| listmonk | `./cmd` | true | false | 2598 | 1607747208 |  |
| pocketbase | `./examples/base` | true | false | 5536 | 2737837008 |  |
| caddy | `./cmd/caddy` | true | false | 1463 | 2593746848 |  |
| gitea | `.` | true | false | 23148 | 6278271176 |  |
| mattermost | `./cmd/mattermost` | true | false | 3543 | 4117713104 |  |

## Unsupported First Blockers

| Edge kind | Count |
|---|---:|
| StructFieldFuncValue | 22 |
| Unsupported | 1 |

## Gap Analysis

- `StructFieldFuncValue` blocks 22 trace(s). Follow-up augmentation should start with the concrete patterns listed in the miss details below.
- `Unsupported` blocks 1 trace(s). Follow-up augmentation should start with the concrete patterns listed in the miss details below.
- `mattermost` has 1 target-not-found trace(s); verify package patterns, build tags, and nested modules before treating those as graph-edge misses.

## Trace Miss Details

### caddy/M-1

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `cmd/main.go:72` to `cmd/commandfuncs.go:172`, func `cmdRun`
- Raw: `cmd/main.go:72` `defaultFactory.Build().Execute()` -> `cmd/commandfuncs.go:172` `cmdRun`

### caddy/M-2

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `cmd/main.go:72` to `cmd/commandfuncs.go:172`, func `cmdRun`
- Raw: `cmd/main.go:72` -> `cmd/commandfuncs.go:172` `cmdRun`

### caddy/M-3

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `cmd/main.go:72` to `cmd/commandfuncs.go:172`, func `cmdRun`
- Raw: `cmd/main.go:72` (`...Execute()`) -> `cmd/commandfuncs.go:172` `cmdRun`

### caddy/M-4

- Category: `unsupported-edge-kind`
- First blocker: step 2 `init-populated-registry` (Unsupported)
- Pattern: from `cmd/main.go:72` to `cmd/commands.go:166`, func ``
- Raw: `cmd/main.go:72` (`defaultFactory.Build()`) -> `cmd/commands.go:166` (`cmd.RunE = WrapCommandFuncForCobra(cmdRun)`)

### caddy/M-5

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `cmd/main.go:72` to `cmd/commandfuncs.go:172`, func `cmdRun`
- Raw: `cmd/main.go:72` `defaultFactory.Build().Execute()` -> `cmd/commandfuncs.go:172` `cmdRun`

### caddy/M-7

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `cmd/main.go:72` to `cmd/commandfuncs.go:172`, func `cmdRun`
- Raw: `cmd/main.go:72` -> `cmd/commandfuncs.go:172` `cmdRun`

### gitea/M-13

- Category: `unsupported-edge-kind`
- First blocker: step 1 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `cmd/main.go:160` to `cmd/web.go:251`, func `runWeb`
- Raw: `cmd/main.go:160` -> `cmd/web.go:251` `runWeb`

### gitea/M-16

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `cmd/main.go:160` to `cmd/admin_user_change_password.go:47`, func `runChangePassword`
- Raw: `cmd/main.go:160` (`app.Run`) -> `cmd/admin_user_change_password.go:47` `runChangePassword`

### mattermost/M-1

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `server/cmd/mattermost/commands/root.go:17` to `server/cmd/mattermost/commands/server.go:39`, func `serverCmdF`
- Raw: `server/cmd/mattermost/commands/root.go:17` `RootCmd.Execute()` -> `server/cmd/mattermost/commands/server.go:39` `serverCmdF` (via `serverCmd.RunE` set at `server.go:30`)

### mattermost/M-10

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `server/cmd/mattermost/commands/root.go:17` to `server/cmd/mattermost/commands/server.go:39`, func `serverCmdF`
- Raw: `server/cmd/mattermost/commands/root.go:17` -> `server/cmd/mattermost/commands/server.go:39` `serverCmdF`

### mattermost/M-11

- Category: `unsupported-edge-kind`
- First blocker: step 3 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `vendor/github.com/spf13/cobra/command.go:1015` to `server/cmd/mattermost/commands/import.go:104`, func `bulkImportCmdF`
- Raw: `vendor/github.com/spf13/cobra/command.go:1015` -> `server/cmd/mattermost/commands/import.go:104` `bulkImportCmdF`

### mattermost/M-12

- Category: `unsupported-edge-kind`
- First blocker: step 1 `init-time-function-field-dispatch` (StructFieldFuncValue)
- Pattern: from `server/cmd/mattermost/main.go:20` to `server/cmd/mattermost/commands/server.go:39`, func `serverCmdF`
- Raw: `server/cmd/mattermost/main.go:20` (`commands.Run` → `RootCmd.Execute`) -> `server/cmd/mattermost/commands/server.go:39` `serverCmdF`

### mattermost/M-13

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `server/cmd/mattermost/commands/root.go:17` to `server/cmd/mattermost/commands/server.go:39`, func `serverCmdF`
- Raw: `server/cmd/mattermost/commands/root.go:17` -> `server/cmd/mattermost/commands/server.go:39` `serverCmdF`

### mattermost/M-14

- Category: `unsupported-edge-kind`
- First blocker: step 3 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `` to `server/cmd/mattermost/commands/server.go:39`, func `serverCmdF`
- Raw: `(*cobra.Command).Execute` body (`RunE` field load) -> `server/cmd/mattermost/commands/server.go:39` `serverCmdF`

### mattermost/M-15

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `server/cmd/mattermost/commands/root.go:17` to `server/cmd/mattermost/commands/import.go:52`, func `slackImportCmdF`
- Raw: `server/cmd/mattermost/commands/root.go:17` (`RootCmd.Execute()`) -> `server/cmd/mattermost/commands/import.go:52` `slackImportCmdF`

### mattermost/M-2

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `server/cmd/mattermost/commands/root.go:17` to `server/cmd/mattermost/commands/server.go:39`, func `serverCmdF`
- Raw: `server/cmd/mattermost/commands/root.go:17` `RootCmd.Execute()` -> `server/cmd/mattermost/commands/server.go:39` `serverCmdF`

### mattermost/M-3

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `server/cmd/mattermost/commands/root.go:17` to `server/cmd/mattermost/commands/server.go:39`, func `serverCmdF`
- Raw: `server/cmd/mattermost/commands/root.go:17` (`RootCmd.Execute()`) -> `server/cmd/mattermost/commands/server.go:39` `serverCmdF`

### mattermost/M-4

- Category: `target-not-found`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `server/cmd/mattermost/commands/root.go:17` to `server/cmd/mattermost/commands/server.go:39`, func `serverCmdF`
- Raw: `server/cmd/mattermost/commands/root.go:17` (`RootCmd.Execute()`) -> `server/cmd/mattermost/commands/server.go:39` `serverCmdF`

### mattermost/M-5

- Category: `unsupported-edge-kind`
- First blocker: step 3 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `` to `server/cmd/mattermost/commands/export.go:123`, func `bulkExportCmdF`
- Raw: `(external) cobra dispatch` -> `server/cmd/mattermost/commands/export.go:123` `bulkExportCmdF`

### mattermost/M-6

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `server/cmd/mattermost/commands/root.go:17` to `server/cmd/mattermost/commands/server.go:39`, func `serverCmdF`
- Raw: `server/cmd/mattermost/commands/root.go:17` -> `server/cmd/mattermost/commands/server.go:39` `serverCmdF`

### mattermost/M-7

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `server/cmd/mattermost/commands/root.go:17` to `server/cmd/mattermost/commands/server.go:39`, func `serverCmdF`
- Raw: `server/cmd/mattermost/commands/root.go:17` `RootCmd.Execute()` -> `server/cmd/mattermost/commands/server.go:39` `serverCmdF`

### mattermost/M-8

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `server/cmd/mattermost/commands/root.go:17` to `server/cmd/mattermost/commands/server.go:39`, func `serverCmdF`
- Raw: `server/cmd/mattermost/commands/root.go:17` `RootCmd.Execute()` -> `server/cmd/mattermost/commands/server.go:39` `serverCmdF`

### mattermost/M-9

- Category: `unsupported-edge-kind`
- First blocker: step 2 `function-value-in-struct-field` (StructFieldFuncValue)
- Pattern: from `server/cmd/mattermost/commands/root.go:17` to `server/cmd/mattermost/commands/server.go:39`, func `serverCmdF`
- Raw: `server/cmd/mattermost/commands/root.go:17` `RootCmd.Execute()` (RunE set at `commands/server.go:36`) -> `server/cmd/mattermost/commands/server.go:39` `serverCmdF`
