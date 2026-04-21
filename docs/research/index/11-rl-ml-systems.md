# 11 — Reinforcement Learning for Systems Decisions

All papers here are candidate templates for replacing hand-tuned delegate expressions
with a learned placement policy. See also [07-scheduling](07-scheduling.md) §ML-based
microservice management for Sage, Sinan, FIRM, and Cilantro.

---

## Pensieve

**Source:** `inspiration/papers/pensieve-sigcomm17.pdf`

Mao, Netravali, Alizadeh (SIGCOMM '17). Replaced hand-tuned adaptive-bitrate heuristics
in video streaming with an A3C-trained RL agent, improving QoE 12–25%.

**Canonical lesson** (`docs/research/claude_comprehensive_research.md` line 107):
*"An RL agent can learn a better threshold than you can tune by hand."*
Pensieve is the intellectual template for replacing hand-written delegate expressions
with a learned policy.

---

## Decima

**Source:** `inspiration/papers/decima-sigcomm19.pdf`

Mao, Schwarzkopf, Venkatakrishnan, Meng, Alizadeh (SIGCOMM '19). GNN + RL for DAG
job scheduling; 21% lower average JCT than Spark's default scheduler.

**For Monolift**: The lift graph is a DAG. Decima's approach — learn a GNN over the
job DAG to make scheduling decisions — is directly analogous to learning a policy
over the lift call graph to make placement decisions.

---

## AuTO

**Source:** `inspiration/papers/auto-sigcomm18.pdf`

Chen, Lan, Chen, Zhang, Wu, Chen (SIGCOMM '18). Coflow scheduling with DDPG.
Online RL agent managing bandwidth allocation for coflows in a datacenter.

---

## Park

**Source:** `inspiration/papers/park-neurips19.pdf`

Mao et al. (NeurIPS '19). Packages twelve systems problems as RL benchmarks.

**Critical warning** (`docs/research/claude_comprehensive_research.md` line 107):
Park explicitly catalogues four pitfalls of RL in production systems:
1. Slow convergence
2. Reward mis-specification
3. Safety under exploration
4. Distribution shift

These are exactly the pitfalls Monolift would face with RL-based delegate expressions.

---

## FIRM (cross-reference)

**Source:** `inspiration/papers/firm-osdi20.pdf`

Qiu, Banerjee, Jha, Kalbarczyk, Iyer (OSDI '20). Two-level RL: SVM identifies
the critical path, DDPG re-allocates resources to SLO-violating services.

See [07-scheduling](07-scheduling.md) §ML-based microservice management.

---

## The honest conclusion for Monolift

`docs/research/claude_comprehensive_research.md` lines 108–109:

> *"RL-based placement is a 2027 thesis, not a 2026 workshop paper. A learned policy
> that ignores a production constraint like 'don't flap the payment service at checkout'
> can cost money."*

**Tractable near-term alternative**: *offline imitation learning* from a hand-tuned
delegate expression. Train a differentiable policy to replicate the hand-tuned behavior;
the result is auditable against the original rules and can serve as a differentiable
proxy for the cost model inside the delegate expression DSL.

The theoretical gap this creates: Cilantro (`docs/research/claude_comprehensive_research.md`
line 79; `inspiration/papers/cilantro-osdi23.pdf`) provides the convergence proof template
that delegate-expression composition currently lacks. Cilantro's Thompson-sampling bandit
proof is the style of stability argument Monolift needs.

---

## Implications for delegate expression design

Synthesizing across Pensieve, Decima, Park, Sinan ([07-scheduling](07-scheduling.md)):

| Design choice | Empirical basis |
|---|---|
| Target p99, not mean latency | Dean & Barroso §Tail at Scale; fan-out amplification |
| Schmitt hysteresis in threshold logic | HPA practice; Pensieve confirms hand-tuning is hard |
| Cooldown periods and rate limits | SEDA [07-scheduling](07-scheduling.md); oscillation analysis |
| Cost model parameterized by live RTT/bandwidth | MAUI empirical finding (30–60% profitable; shifts with network) |
| Offline imitation learning before online RL | Park's four pitfalls |
| Centralized critic or shared signal for N-lift composition | MARL result; [07-scheduling](07-scheduling.md) §oscillation |
