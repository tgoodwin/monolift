# Listmonk Summary

## Scope

- Traces analyzed: 10 (`listmonk/M-1` through `M-10`)
- Codebase size: ~20k LOC
- Dominant path shapes: campaign/message worker pipelines, messenger implementations, media processing, subscriber import, and bounce webhook handlers.

## Surface-Area Pressure

Listmonk is small enough that surface area is not uniformly decisive after the first goroutine or HTTP registration step. Shallow cuts are still poor because they include worker loops, server setup, or handler setup, but the gap between a mid-depth service method and the region root is smaller than in Gitea or Mattermost.

## Dominant Tradeoff Axis

The dominant axis is boundary data plus client reconstruction. SMTP clients, webhook HTTP clients, POP/SNS/Sendgrid bounce inputs, database-backed import sessions, and campaign template state are usually reconstructible from config or request payloads. Once the cut is below the worker or handler shell, most Listmonk traces become feasible with ordinary serialized domain structs.

## Synthesis Notes

- Goroutine launches at `Manager.Run` and worker loops are anti-boundaries; the useful cuts are below them.
- Interface dispatch to messenger implementations (`Emailer.Push`, `Postback.Push`) provides strong natural boundaries with small extraction surface.
- Because the codebase is compact, error semantics and state reconstruction are better discriminators than raw LOC moved.
