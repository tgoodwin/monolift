# Archetype Catalog v1 - gpt-5.4 run

Status: `v1`.

Cross-links: [narrative note](./distribution-archetypes-v1.md), [follow-ups](./distribution-archetypes-followups.md), [listmonk](./annotations/listmonk.md), [miniflux](./annotations/miniflux.md), [caddy](./annotations/caddy.md), [pocketbase](./annotations/pocketbase.md), [gitea](./annotations/gitea.md), [mattermost](./annotations/mattermost.md).

## Vocabulary Discipline

- Coverage gate: each survivor appears in at least 2 regions across at least 2 targets.
- Evidence gate: each survivor is distinguished by evidence the classifier already has or by one named candidate signal.
- Emission gate: a short Go-shaped transform sketch exists and differs materially from the other survivors.
- Boundary gate: AUTO, SUGGEST, and TERMINAL conditions are stated as evidence conditions, not vibes.
- Naming-collision check: executed against ADR-0015 through ADR-0018 and the v2 contract. These names do not reuse canonical shape or committed state-class terms.

## Baseline Already Admitted

Not counted as new sprint output, but important contrast:

- replicated function or handler over `immutable-captured-config` or `externalized-durable`
- corpus anchors: Caddy reverse proxy handler, Miniflux `ProcessFeedEntries`

## Surviving Archetypes

### Queue-Backed Worker

Definition: a bounded queue feeds one or more workers that handle serializable items independently, with retry or requeue semantics localized in the worker layer.

Gate outcomes:

- coverage: pass
- evidence: pass
- emission: pass
- boundary: pass

Candidate ADR-0016 state class: `queued-workset`

Evidence conditions:

- explicit queue or channel of work items
- item payload can be serialized without carrying live process pointers
- worker body only needs externalized durable state or immutable config
- ordering is not globally load-bearing, or the current code already tolerates buffering/requeue

AUTO threshold:

- all four evidence conditions hold
- retry/requeue semantics are explicit or safely defaultable

SUGGEST threshold:

- the queue shape is clear but ordering, idempotence, or retry semantics are only partially explicit

TERMINAL threshold:

- worker correctness depends on hidden shared mutable state, in-process pointer aliasing, or untracked cross-item coordination

Emission sketch:

```go
type Job struct{ Payload json.RawMessage }

func Enqueue(ctx context.Context, j Job) error {
	return broker.Publish(ctx, "queue-name", j)
}

func Worker(ctx context.Context) error {
	for j := range broker.Consume(ctx, "queue-name") {
		if err := handle(ctx, j); err != nil {
			broker.Retry(ctx, j)
		}
	}
	return nil
}
```

Pragma bridge:

- acceptable: job payload type or handler root selection when multiple worker candidates exist
- not acceptable: forcing AUTO when item independence is not statically visible

Remediation surface:

- "Detected queue-backed worker. Found bounded queue, worker body, and serializable item type. Missing: <ordering/idempotence proof>. Suggested transform: broker-backed queue plus worker replicas."

Corpus citations:

- Listmonk `internal/manager` and `internal/bounce`
- Miniflux `internal/worker`
- Gitea `modules/queue` and indexers
- Mattermost `channels/jobs`

### Scheduled-Reconciler

Definition: a timer, ticker, or cron loop periodically triggers a work body or enqueues work; the cadence, not the work itself, is the part currently tied to one process.

Gate outcomes:

- coverage: pass
- evidence: pass
- emission: pass
- boundary: pass

Candidate ADR-0016 state class: `scheduled-reconciler`

Evidence conditions:

- explicit timer, ticker, cron, or sleep loop
- work body is factored enough to invoke independently
- duplicate or skipped ticks are either already tolerated or can be absorbed by existing de-duplication/idempotence

AUTO threshold:

- cadence is clearly separate from work body
- work body is idempotent, de-duplicated, or already state-externalized

SUGGEST threshold:

- timer loop is clear, but missed/duplicate tick semantics remain ambiguous

TERMINAL threshold:

- cadence is entangled with connection-local or in-memory ephemeral state that cannot be replayed honestly

Emission sketch:

```go
func Reconcile(ctx context.Context) error {
	return doWork(ctx)
}

func Register() {
	platform.Schedule("job-name", cronExpr, func(ctx context.Context) error {
		return Reconcile(ctx)
	})
}
```

Pragma bridge:

- acceptable: job name, cadence choice, or root narrowing when several timer loops exist
- not acceptable: pragma claims that duplicate or missed ticks are safe when source evidence says otherwise

Remediation surface:

- "Detected scheduled reconciler. Found explicit timer loop and separated work body. Missing: <tick safety proof>. Suggested transform: platform scheduler invoking the existing body or a queue enqueuer."

Corpus citations:

- Listmonk bounce mailbox scanner
- PocketBase `tools/cron.Cron`
- Gitea eventsource refresh loop
- Mattermost periodic and daily schedulers

### Serialized-Singleton-Owner

Definition: one mutable owner instance protects process-local state with a mutex or equivalent serialization primitive; the real semantic requirement is single ownership, not local memory.

Gate outcomes:

- coverage: pass
- evidence: pass
- emission: pass
- boundary: pass

Candidate ADR-0016 state class: `owned-mutable-singleton`

Evidence conditions:

- one owner object or package-global owner is identifiable
- mutation goes through a narrow method surface
- state does not need address identity outside the owner
- alias or escape surface is small enough to bound

AUTO threshold:

- owner identity is unique and stable
- mutations are routed through the owner surface
- no load-bearing alias leaks are visible

SUGGEST threshold:

- owner shape is credible, but alias, regeneration, or lifecycle semantics are not yet closed-form

TERMINAL threshold:

- multiple callers intentionally share and mutate state without one owner, or state relies on process-local pointer identity

Emission sketch:

```go
type Request struct{ Op string; Key string; Value any }

func Handle(req Request) (any, error) {
	return rpc.Call("singleton-owner", req)
}

func OwnerLoop(ctx context.Context) error {
	state := newState()
	for req := range rpc.Receive(ctx) {
		reply(req, state.Apply(req))
	}
	return nil
}
```

Pragma bridge:

- acceptable: naming the intended owner root when several mutexed structs exist
- not acceptable: overriding visible alias leakage or multi-owner mutation

Remediation surface:

- "Detected serialized singleton owner. Found one mutable owner plus serialized mutation surface. Missing: <alias/lifecycle proof>. Suggested transform: single remote owner with RPC serialization replacing the local lock."

Corpus citations:

- Listmonk `internal/tmptokens`
- Miniflux proxy rotator
- PocketBase hook registries
- Gitea session providers

### Connection-Hub-Buffer

Definition: a per-user or per-connection hub owns subscriber registration, buffered delivery, reconnect identity, or replay state; the semantic requirement is sticky session ownership plus explicit fanout behavior.

Gate outcomes:

- coverage: pass
- evidence: pass
- emission: pass
- boundary: pass

Candidate ADR-0016 state class: `connection-hub-buffer`

Evidence conditions:

- explicit connection or subscriber identity
- register/unregister API or equivalent ownership surface
- bounded active/dead queue, replay, or fanout buffer semantics are visible
- a stable routing key exists for sticky ownership

AUTO threshold:

- all four evidence conditions hold
- ordering and replay behavior are explicit in source

SUGGEST threshold:

- subscriber registry exists, but replay, loss, or backpressure semantics are not explicit enough

TERMINAL threshold:

- fanout depends on hidden process-local pointers or on subscriber semantics that are only runtime-conventional

Emission sketch:

```go
type ConnKey struct{ UserID, ConnID string }

func Register(k ConnKey) error {
	return hub.Register(k)
}

func Publish(k ConnKey, msg Event) error {
	return hub.Send(k.UserID, msg)
}

func Replay(k ConnKey, seq int64) ([]Event, error) {
	return hub.Recover(k, seq)
}
```

Pragma bridge:

- acceptable: naming the routing key (`user_id`, `connection_id`) when both are present
- not acceptable: forcing AUTO when replay or loss semantics are not represented in source

Remediation surface:

- "Detected connection hub with bounded fanout/replay state. Missing: <loss or replay rule>. Suggested transform: sticky-owned hub service with externalized replay buffer."

Corpus citations:

- PocketBase `tools/subscriptions.Broker`
- Gitea eventsource manager/messenger
- Mattermost websocket hub and reliable queues
- Listmonk local events bus as the SUGGEST boundary case

## Retired / Merged Candidates

### Pipeline Stage

Retired as an independent v1 term. The corpus did show stage-like queue boundaries, but every credible example collapsed into `queue-backed worker` once I applied the emission gate. The generated shape was still "serialize item, enqueue, drain in a worker." Keeping a separate label would only restate topology, not transform.

### Sharded Keyed State

Retired for v1 on coverage grounds. The motivating example is plausible, but the pinned corpus in this run did not produce enough clean, line-level keyed-ownership examples to justify a separate term. Session maps and per-user hubs looked closer to `connection-hub-buffer` or `serialized-singleton-owner` than to general shard routing.

### Distributed Cache Wrapper

Merged into `serialized-singleton-owner` or left terminal. The corpus did show caches and local stores, but the transform boundary was never "generic distributed cache" in a way that produced distinct evidence conditions from a single-owner service or from decorated persistence.

### Config / Control Plane

Retired as a remediation archetype. PocketBase and Mattermost both contain large config and orchestration layers, but those regions were valuable mostly as terminal contrast. They are real architecture, but not the next auto-lift surface for Monolift.
