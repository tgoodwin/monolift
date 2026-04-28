# ADR-0026: Boot-path extraction

**Status:** accepted
**Date:** 2026-04-27
**Related:** ADR-0017, ADR-0018, ADR-0022, ADR-0025

## Context

Long-lived extracted services need more than a lifted function body. They need
the startup context that created the region: config sources, initialization
order, and long-lived goroutines that belong with the region.

## Decision

The compiler adds a boot-path extraction pass after admission and
surface-derivation, before emission.

The pass emits a `BootSpec`:

- `ConfigSource`: env, flag, file, literal, or report-only DB source.
- `DependencyInit`: required, substitutable, or disabled-by-minimal-config.
- `GoroutineLaunch`: long-lived launch sites that must run in the extracted
  service.
- `BootPathRefusal`: emission blockers. These do not retroactively change
  admission.

Manifest emission consumes `BootSpec` config sources. Secret-like names route to
Kubernetes Secret references; ordinary env/flag/file sources route through
ConfigMaps and volume mounts.

## Consequences

Boot-path failure is an emission characterization, not an ADR-0017 liftability
refusal. Mattermost proved the pass is bounded at target scale, but also showed
the current reverse-path modeling needs route/constructor reachability before it
can honestly emit Mattermost.
