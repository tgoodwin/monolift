# Listmonk Annotation & Coverage Ledger

## Target Information
- **Name**: Listmonk
- **Total Go Files**: 92

## Coverage Ledger

| Bundle | Subsystem | Directories | File Count | Status |
| :--- | :--- | :--- | :--- | :--- |
| **L-ALL** | full | `.` | 92 | DONE |

## Annotations

### L-MGR-001: Listmonk Manager
- **subsystem**: core
- **owned directories**: `manager/`
- **region or operation identity**: `github.com/knadh/listmonk/internal/manager:Manager` (type)
- **admitted or refused**: refused (assumed)
- **triage**: AUTO
- **proposed archetype**: worker-pool
- **proposed candidate state class**: singleton-mutable
- **proposed transform**: Externalize `campMsgQ` and `msgQ` to a distributed queue; scale `Manager` workers as a separate service tier.
- **competing archetypes considered**: singleton-actor (for template caching)
- **evidence signals seen**: multiple `chan` members, `sync.RWMutex` for shared maps (`pipes`, `tpls`, `links`), background worker pattern.
- **missing evidence**: Serialization logic for `CampaignMessage` and `models.Message` if they contain complex interfaces.
- **file references**: `evaluation/listmonk/internal/manager/manager.go:64`
