# Knative and Vercel Fluid Compute Concepts Applied to Monolift

Date: February 17, 2026  
Scope: Public Knative and Vercel documentation, mapped to Monolift's generated local/remote delegation model.

## Executive Summary

Monolift already has the core primitive that both Knative and Fluid Compute rely on: route each request to one of multiple execution targets based on runtime state. In Monolift, this is the generated delegate that picks `local` vs `remote` using a decider. The biggest opportunity is to evolve that binary decision into a policy/control loop with queueing, weighted traffic, and failure-aware fallback.

The practical direction is:

1. Upgrade decisioning from threshold checks to a small control plane loop (hysteresis + backlog + remote health).
2. Add progressive traffic and burst handling (like Knative's traffic split and autoscaling targets).
3. Add efficiency policies for long-running and IO-heavy paths (similar to Fluid's concurrency and active CPU framing).

## Monolift Baseline (From Current Code)

Monolift currently generates delegate wrappers that call either a local service implementation or a remote client:

- Delegate generation: `pkg/lift/templates/delegate.go.tmpl`
- Delegate wiring into entrypoint: `pkg/compiler/compiler.go`
- Decider types: `pkg/pragma/resolver.go`
- Decider interface and controller: `pkg/pragma/controller.go`

Current behavior:

- A delegate calls `decider.ShouldDelegate()` in the hot path.
- Decider signals include CPU, MEM, and IPS thresholds.
- The compiler generates remote client wiring and decider initialization for extracted services.
- There is a `ThresholdController` concept for hysteresis/stateful control, but the core update loop is still marked TODO in `pkg/pragma/controller.go`.

This means Monolift has a strong request-routing primitive, but not yet a full feedback control system.

## Relevant Knative Concepts

### 1) Activator + Queue Proxy + Autoscaler as a control loop

Knative Serving's request flow and autoscaling docs describe a data-plane path with queueing and a control loop reacting to traffic and concurrency. This is conceptually similar to Monolift's decider, but richer: it includes buffering and scaling decisions, not just a per-call binary route.

How this maps:

- Monolift decider -> Knative autoscaler decision signal
- Monolift remote path -> Knative scaled service path
- Missing in Monolift today -> queue-depth and burst-aware admission behavior

### 2) Container concurrency and autoscaling target

Knative allows per-service/revision concurrency controls and autoscaling target values. This gives operators explicit control over latency vs efficiency tradeoffs.

How this maps:

- Monolift can add per-extracted-service `target_inflight`, `max_inflight`, and `burst_capacity` settings.
- Delegation can trigger on in-flight/backlog in addition to CPU/MEM/IPS.

### 3) Revisions and traffic splitting

Knative supports routing percentages across revisions. This is directly useful for safe rollout.

How this maps:

- Monolift can support weighted local/remote routing (`90/10`, `50/50`, etc.) instead of hard threshold cutover.
- This gives canary extraction behavior and rollback without regeneration.

### 4) Eventing broker/trigger patterns

Knative Eventing introduces brokered, trigger-based delivery and decoupled async flow.

How this maps:

- Monolift could support async delegation for operations that are not strictly request-response critical, reducing p95 impact during spikes.

## Relevant Vercel Fluid Compute Concepts

### 1) Instance reuse and in-function concurrency

Fluid emphasizes higher utilization by reusing warm instances and allowing multiple invocations when runtime behavior permits. It also highlights isolation concerns when using shared global state.

How this maps:

- Monolift can improve remote counterpart efficiency by using bounded concurrency pools rather than one-request-at-a-time assumptions.
- Monolift should define clear state-safety guidance for extracted services if concurrency is increased.

### 2) Dynamic scaling and reduced cold-start impact

Fluid positions itself around burst handling and reducing cold-start penalties via runtime behavior and placement.

How this maps:

- Monolift can add warm-remote policies (`min_warm_remotes`) and preconnect remote clients before delegation ramps up.
- Delegation policy should include cold-start penalty in route scoring.

### 3) Invocation failover and resilience

Fluid documentation describes resilience-oriented execution handling.

How this maps:

- Monolift delegates should have explicit fallback semantics:
  - remote timeout/error -> immediate local fallback when safe
  - optional retry budget and circuit breaker per extracted service

### 4) Active CPU framing for cost/perf

Fluid documents emphasize active CPU usage as a billing/performance framing.

How this maps:

- Monolift policy can optimize for "active compute saved" rather than only host utilization thresholds.
- This is especially useful for IO-heavy endpoints where wall-clock duration can be misleading.

## Conceptual Overlap Matrix

| Problem | Knative Concept | Fluid Concept | Monolift Equivalent / Opportunity |
|---|---|---|---|
| Overload routing | Autoscaler + activator | Dynamic instance scaling | Decider + delegate (extend with backlog signal) |
| Safe rollout | Revision traffic split | Runtime feature enablement | Weighted local/remote delegation |
| Cold starts | Scale-from-zero handling | Instance reuse, optimized starts | Warm remote counterparts + preconnect |
| Efficiency | Concurrency targets | In-function concurrency | Bounded concurrent remote delegation |
| Resilience | Queueing + service retries | Invocation failover | Local fallback + circuit breaker |

## Recommended Monolift Adaptation Plan

### Phase 1: Control-loop hardening (short term)

1. Implement `ThresholdController.updateState()` with real hysteresis windows in `pkg/pragma/controller.go`.
2. Add per-service in-flight and queue-depth signals to decider input.
3. Add remote health probe and circuit breaker; degrade to local path on remote instability.

Expected result: fewer route oscillations, better tail latency under burst.

### Phase 2: Progressive traffic and burst policies (mid term)

1. Introduce weighted delegation policy (`local_weight`, `remote_weight`) for canary extraction.
2. Add config for `target_inflight`, `max_inflight`, and `burst_capacity`.
3. Add warm-remote policy (`min_warm_remotes` or preconnect behavior).

Expected result: safer extraction rollouts and better load-shaping.

### Phase 3: Async delegation mode and efficiency objective (longer term)

1. Support async offload pattern for non-critical paths (broker/trigger style).
2. Add policy objective modes:
   - `latency_first`
   - `cost_efficiency` (active CPU style)
   - `balanced`

Expected result: broader applicability across endpoint types and better cost/perf control.

## Suggested Policy Surface (Example)

```yaml
monolift:
  services:
    timelineservice.Service:
      mode: balanced
      signals:
        cpu_threshold: 0.80
        mem_threshold: 0.85
        ips_threshold: 100
        target_inflight: 40
        burst_capacity: 120
      traffic:
        local_weight: 90
        remote_weight: 10
      resilience:
        remote_timeout_ms: 120
        breaker_error_rate: 0.10
        fallback_to_local: true
      warmup:
        min_warm_remotes: 1
```

## Sources

- Knative request flow: [https://knative.dev/docs/serving/request-flow/](https://knative.dev/docs/serving/request-flow/)
- Knative autoscaling concepts: [https://knative.dev/docs/serving/autoscaling/](https://knative.dev/docs/serving/autoscaling/)
- Knative autoscaling target and class: [https://knative.dev/docs/serving/autoscaling/autoscaling-targets/](https://knative.dev/docs/serving/autoscaling/autoscaling-targets/)
- Knative traffic management: [https://knative.dev/docs/serving/traffic-management/](https://knative.dev/docs/serving/traffic-management/)
- Knative eventing broker/trigger: [https://knative.dev/docs/eventing/broker/](https://knative.dev/docs/eventing/broker/)
- Vercel Fluid Compute docs: [https://vercel.com/docs/functions/fluid-compute](https://vercel.com/docs/functions/fluid-compute)
- Vercel in-function concurrency guide: [https://vercel.com/guides/what-can-i-do-about-vercel-serverless-functions-timing-out](https://vercel.com/guides/what-can-i-do-about-vercel-serverless-functions-timing-out)
- Vercel Fluid pricing/compute framing: [https://vercel.com/docs/pricing/serverless-functions](https://vercel.com/docs/pricing/serverless-functions)
- Vercel changelog context: [https://vercel.com/changelog/more-flexible-serverless-compute-with-fluid](https://vercel.com/changelog/more-flexible-serverless-compute-with-fluid)
