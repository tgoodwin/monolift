# Can Monolift's refusal codes can guide refactoring?

Monolift's [refusal codes](../refusal-diagnostics.md) name *why* a
region cannot be lifted (shared mutable state, unliftable receiver,
etc.). Those error messages don't reflect "compile time errors" in the traditional sense, but reflect properties of functioning code that prevent it from being distribut-able.
These error codes could thus be used as guidance for developers to enable greater "liftability" in their codebases by improving internal modularity.

A successful refactor in presence of error would be evidence that the compiler's diagnostics are not just honest about failure, but actionable, i.e. helping a *determined user* nudge their codebase toward more modularity along exactly the axes distribution requires (much cheaper than rewriting to a whole new framework like ServiceWeaver).

How might we evaluate a *determined user*? ...Agents! We could pair Monolift with an LLM agent and evaluate # of LOC changed by the agent to unlock a lift, wall time, etc. Similar to how we gave agents access to Kamera to find bugs. This exercise would effectively measure the utility of Monolift's error code mechanisms (i.e. the extent to which they are actionable). Like Kamera, Monolift itself is not about AI agents, but it could be interesting to illustrate how Monolift, as a deterministic, readily-reversible code manipulation tool, can aid agentic coding / refactoring.
