# Distribution Archetypes: Follow-up Research Items

## 1. Idempotency Guardrails for Auto-Lifting
Many archetypes (Worker Pool, Scheduled Invocation) assume that the work being performed is idempotent or that exactly-once delivery is not required.
- **Task**: Research static analysis patterns for detecting side-effect idempotency.
- **Signal**: DB transaction usage, `UPSERT` patterns, `OnConflict` clauses.

## 2. Distributed Locking Archetype
Some `singleton-actor` instances need to be distributed but still require mutual exclusion across multiple replicas (e.g., a "Leader" worker).
- **Task**: Define a `Leader Actor` archetype that utilizes distributed locks (etcd/consul/redis).
- **Signal**: `if isLeader { ... }` blocks or explicit leadership election libraries.

## 3. Serialization Surface for Job Queues
Worker pools often pass complex objects through channels.
- **Task**: Map the serializability of `model.*` structs in Gitea and Mattermost.
- **Problem**: Pointers to global state or local file handles in job payloads.

## 4. Managed Adapter Metadata
The `Distributed Cache` archetype needs a way to tell the compiler which backend to use.
- **Task**: Extend the `monolift-v2-contract` to include `adapter` metadata in pragmas.
- **Example**: `//monolift:lift archetype=distributed-cache adapter=redis`.
