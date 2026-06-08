# Can Monolift's refusal codes can guide refactoring?

Monolift's [refusal codes](../refusal-diagnostics.md) name *why* a
region cannot be lifted (shared mutable state, unliftable receiver,
etc.). That naming carries more signal than a generic compile error:
each code points at a structural property of the source code that, if
changed, would unblock the lift.

The question this page evaluates: can those refusal codes drive **targeted refactors** that turn a refused
region into an admitted one — without changing observable behavior?
A successful refactor would be evidence that the compiler's
diagnostics are not just honest about failure, but actionable -- helping a determined user
nudge their codebase toward more modularity along exactly the axes distribution requires (much cheaper than rewriting to a whole new framework like ServiceWeaver).

We could pair Monolift with an LLM agent and evaluate # of LOC changed by the agent to unlock a lift.

!!! note "Stub"
    Content to be filled in.
