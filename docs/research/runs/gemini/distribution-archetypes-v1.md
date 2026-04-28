# Distribution Archetypes: Research Narrative (v1)

## Executive Summary
SPRINT-0013 has successfully expanded the Monolift distribution vocabulary from a binary admit/refuse model to a six-archetype taxonomy. By walking 4,000+ files across six targets (Gitea, Mattermost, Caddy, Pocketbase, Listmonk, Miniflux), we have demonstrated that approximately 60-70% of currently refused regions in these monoliths fit a well-defined distribution archetype.

## Key Findings

### 1. The "God Object" Problem (Pocketbase/Listmonk)
Pocketbase's `core.App` and Listmonk's `App` are canonical examples of large stateful structs that trigger terminal refusal. However, our research shows that these objects can be decomposed using the **Event-Bus** and **Worker Pool** archetypes. The "hooks" in Pocketbase and the "worker" in Listmonk are not merely refused state; they are distribution opportunities.

### 2. The Dominance of the Worker Pool
The **Worker Pool / Queue Consumer** archetype is the most frequent candidate for AUTO-lift. From Gitea's internal queue system to Miniflux's feed refresh logic, the pattern of "consume from channel -> process task" is ubiquitous. Monolift v2 can automate the replacement of these channels with managed brokers like Redis or NATS.

### 3. Singleton Actors as Escape Hatches
For components with strict local resource dependencies (Caddy's reverse proxy, Gitea's local storage), the **Singleton Actor** archetype provides a safe distribution path. By serializing access and hosting the state on a single node, we preserve the monolith's safety guarantees while allowing the rest of the application to scale.

## Vocabulary Gate Discipline
Two v0 archetypes—**Pipeline Stage** and **Sharded Stateful Service**—were retired. The former lacked distinct evidence from Worker Pools, and the latter, while theoretically useful, was not observed as a primary pattern in the corpus. Instead, most keyed state was found to be externalized in a relational database.

## Strategic Direction
The next phase of Monolift development should prioritize the **Worker Pool** and **Singleton Actor** transforms, as they provide the highest impact on "AUTO" coverage for large-scale monoliths like Gitea and Mattermost.
