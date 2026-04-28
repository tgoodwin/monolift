# Distribution Archetype Catalog (v1)

This catalog defines the distribution archetypes used by the Monolift compiler.

## v1 Catalog

### Singleton Actor
- **Definition**: A stateful component where all access must be serialized to protect internal consistency.
- **Targets**: Listmonk (Core), Caddy (UsagePool), PocketBase (App), Mattermost (App).
- **Transform**: Emit a single service instance; serialize calls at the proxy/stub.
- **Candidate State Class**: `singleton-mutable-mutex`, `singleton-mutable-embedded-db`.

### Worker Pool / Queue Consumer
- **Definition**: Multiple identical workers reading from a shared queue.
- **Targets**: Listmonk (Manager), Miniflux (Pool), Gitea (Queue), Mattermost (Jobs).
- **Transform**: Broker-backed queue feeding a pool of worker services.
- **Candidate State Class**: `worker-pool-channel`.

### Replicated Stateless Service
- **Definition**: No internal state between requests; fully horizontal scale.
- **Targets**: Caddy (Adapters), Listmonk (Messenger), Miniflux (Storage).
- **Transform**: N replicas behind a standard load balancer.
- **Candidate State Class**: `stateless`, `stateless-adapter`.

### Sharded Stateful Service
- **Definition**: State is partitioned by a key; requests are routed based on that key.
- **Targets**: Gitea (Cache), Mattermost (Cache).
- **Transform**: Sharded service cluster; key-hash determined ownership.
- **Candidate State Class**: `singleton-mutable-sharded`.

### HTTP Handler
- **Definition**: Standard ingress-compatible request handler.
- **Targets**: Caddy (Handler), Miniflux (API), Mattermost (api4).
- **Transform**: Standard K8s Ingress / Service.
- **Candidate State Class**: `stateless-http`.

### Ephemeral Worker
- **Definition**: Short-lived task with no long-term state.
- **Targets**: Gitea (Tasks).
- **Transform**: K8s Job.
- **Gate Status**: *Marginal* (needs one more target).

---

## Retired Archetypes

### Pipeline Stage
- **Why it didn't survive**: No clear instances found in the corpus that weren't better described as Worker Pools or Stateless Services. Complexity of chaining services didn't justify a separate archetype yet.

### Event-Bus Publisher / Subscriber
- **Why it didn't survive**: Most internal "pub/sub" patterns seen were actually Worker Pools (queue-based) or could be handled by Singleton Actor serialization. Explicit pub/sub was rare in the analyzed monolith regions.

### Session-Scoped State
- **Why it didn't survive**: While present in targets, it's usually handled by a database or external store, making the handler itself "Stateless". Not enough evidence for a custom *distribution* transform vs. just using a DB.
