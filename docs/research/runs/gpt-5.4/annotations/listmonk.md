# Listmonk Annotation Notes - gpt-5.4 run

Catalog links: [queue-backed worker](../archetype-catalog-v1.md#queue-backed-worker), [scheduled-reconciler](../archetype-catalog-v1.md#scheduled-reconciler), [serialized-singleton-owner](../archetype-catalog-v1.md#serialized-singleton-owner), [connection-hub-buffer](../archetype-catalog-v1.md#connection-hub-buffer).

## Target Synthesis

Listmonk is the cleanest small-target argument for the sprint thesis. Its strongest AUTO surface is not the HTTP app; it is the background machinery hidden behind campaign delivery, bounce ingestion, batch importing, and token cleanup. The target contains three repeating v1-worthy shapes:

- queue-fed workers that consume serializable units of work from bounded channels
- timer-driven reconcilers that can become platform schedulers or scheduled enqueuers
- singleton-owned mutable maps whose in-process locks are standing in for single-owner serialization

Headline AUTO set:

- `internal/manager.Manager` campaign worker loop
- `internal/bounce.Manager.Run` queue consumer
- `internal/bounce.Manager.runMailboxScanner` timed enqueuer
- `internal/tmptokens` global token table plus periodic cleanup
- `internal/subimporter` buffered importer / commit loop

Headline SUGGEST set:

- `internal/events.Events` local fanout bus, because subscriber lifecycle and drop semantics are only partially explicit

## Coverage Ledger

| Bundle | Status | Note |
|---|---|---|
| `cmd` | no relevant archetype surface observed | Mostly HTTP/bootstrap/orchestration; distribution-relevant behavior lives in `internal/*`. |
| `internal/manager` | findings | Campaign queues and worker loop are the strongest queue-backed worker evidence in the target. |
| `internal/bounce` | findings | One queue consumer plus one timer-driven mailbox scan loop. |
| `internal/tmptokens` | findings | Global mutex-protected token map plus background cleanup goroutine. |
| `internal/events` | findings | Local subscriber registry and fanout channels; good SUGGEST pressure test for hub vocabulary. |
| `internal/subimporter` | findings | Buffered import queue with batching and commit cadence. |
| remainder of `internal` | no relevant archetype surface observed | Mostly helpers, formatting, DB wrappers, or request-time code rather than independent distribution surfaces. |

## Region Findings

### Region 1

- `subsystem`: campaign delivery
- `owned directories`: `evaluation/listmonk/internal/manager`
- `region or operation identity`: `Manager.Run`, `Manager.worker`, `Manager.newPipe`
- `admitted or refused`: refused today because the live shape is channel plus goroutine ownership
- `triage`: `AUTO`
- `proposed archetype`: `queue-backed worker`
- `proposed candidate state class`: `queued-workset`
- `proposed transform`: replace the in-process `campMsgQ` / `msgQ` worker loop with a durable queue and a worker service that drains independently serializable campaign messages
- `competing archetypes considered`: `scheduled-reconciler`, `serialized-singleton-owner`
- `evidence signals seen`: bounded channels, goroutine workers, serializable message payloads, no load-bearing shared mutable state outside the campaign store
- `missing evidence`: exact ordering guarantees between campaign batches are not documented, but the code already tolerates queue buffering and backpressure
- `file references`: `evaluation/listmonk/internal/manager/manager.go:266`, `evaluation/listmonk/internal/manager/manager.go:463`, `evaluation/listmonk/internal/manager/pipe.go:27`

### Region 2

- `subsystem`: bounce processing
- `owned directories`: `evaluation/listmonk/internal/bounce`
- `region or operation identity`: `Manager.Run`
- `admitted or refused`: refused today because bounce events arrive through a process-local queue
- `triage`: `AUTO`
- `proposed archetype`: `queue-backed worker`
- `proposed candidate state class`: `queued-workset`
- `proposed transform`: externalize bounce events into a broker-backed queue and keep `RecordBounceCB` as the worker body
- `competing archetypes considered`: `connection-hub-buffer`
- `evidence signals seen`: buffered channel, independent `models.Bounce` items, side effects already funneled through callback and DB write
- `missing evidence`: none material for v1
- `file references`: `evaluation/listmonk/internal/bounce/bounce.go:118`, `evaluation/listmonk/internal/bounce/bounce.go:147`

### Region 3

- `subsystem`: timed reconciliation
- `owned directories`: `evaluation/listmonk/internal/bounce`
- `region or operation identity`: `Manager.runMailboxScanner`
- `admitted or refused`: refused today because the timing loop is process-local
- `triage`: `AUTO`
- `proposed archetype`: `scheduled-reconciler`
- `proposed candidate state class`: `scheduled-reconciler`
- `proposed transform`: move the mailbox scan trigger to an external scheduler that invokes the same scan body or enqueues scan work
- `competing archetypes considered`: `queue-backed worker`
- `evidence signals seen`: infinite loop, explicit sleep interval, work body already separated from delivery worker
- `missing evidence`: none material if duplicate scans are acceptable
- `file references`: `evaluation/listmonk/internal/bounce/bounce.go:135`

### Region 4

- `subsystem`: one-time token store
- `owned directories`: `evaluation/listmonk/internal/tmptokens`
- `region or operation identity`: package-global `tokens` map, `Set`, `Get`, `Clean`, background ticker from `init`
- `admitted or refused`: refused today because mutable singleton state is guarded by a process-local mutex and background goroutine
- `triage`: `AUTO`
- `proposed archetype`: `serialized-singleton-owner`
- `proposed candidate state class`: `owned-mutable-singleton`
- `proposed transform`: emit a single-owner token service and route all token operations through that service; convert cleanup into a scheduled call
- `competing archetypes considered`: `scheduled-reconciler`, `connection-hub-buffer`
- `evidence signals seen`: package-global mutable map, single mutex, bounded API surface, cleanup does not depend on address identity
- `missing evidence`: none material beyond proving no alias to the map escapes
- `file references`: `evaluation/listmonk/internal/tmptokens/tmptokens.go:33`, `evaluation/listmonk/internal/tmptokens/tmptokens.go:46`, `evaluation/listmonk/internal/tmptokens/tmptokens.go:94`, `evaluation/listmonk/internal/tmptokens/tmptokens.go:125`

### Region 5

- `subsystem`: subscriber import
- `owned directories`: `evaluation/listmonk/internal/subimporter`
- `region or operation identity`: buffered subscriber import queue and commit loop
- `admitted or refused`: refused today because queue ownership and batching are in-process
- `triage`: `AUTO`
- `proposed archetype`: `queue-backed worker`
- `proposed candidate state class`: `queued-workset`
- `proposed transform`: externalize `SubReq` buffering and keep the commit batcher as a worker-side accumulation step
- `competing archetypes considered`: `scheduled-reconciler`
- `evidence signals seen`: explicit queue, batch commits, stop channel, worker-style buffering
- `missing evidence`: exact at-most-once expectations around import retries
- `file references`: `evaluation/listmonk/internal/subimporter/importer.go:64`, `evaluation/listmonk/internal/subimporter/importer.go:83`, `evaluation/listmonk/internal/subimporter/importer.go:188`, `evaluation/listmonk/internal/subimporter/importer.go:270`

### Region 6

- `subsystem`: local event bus
- `owned directories`: `evaluation/listmonk/internal/events`
- `region or operation identity`: `Events` subscriber map and `Subscribe`
- `admitted or refused`: refused today because subscriber channels and drop semantics are process-local
- `triage`: `SUGGEST`
- `proposed archetype`: `connection-hub-buffer`
- `proposed candidate state class`: `connection-hub-buffer`
- `proposed transform`: replace local subscriber channels with broker-managed subscription fanout plus bounded reconnect semantics
- `competing archetypes considered`: `queue-backed worker`
- `evidence signals seen`: subscriber registry map, buffered subscriber channels, non-blocking send with explicit queue-full failure
- `missing evidence`: subscriber identity, replay rules, and tolerated loss model are not fully explicit
- `file references`: `evaluation/listmonk/internal/events/events.go:26`, `evaluation/listmonk/internal/events/events.go:41`
