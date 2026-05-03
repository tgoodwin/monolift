# Phase 3 prompt template — activation-path trace

This prompt is fed to a single agent for a single lift-region candidate. Variables in `${...}` are substituted per invocation.

---

You are tracing the **activation path** for a lift-region candidate in the project ${PROJECT_NAME}, in support of the Monolift research project.

**Context.** Monolift is a research compiler that takes a developer-annotated code region and emits both an in-process and a remote-callable implementation. A runtime oracle decides per call which to invoke. For this to work, the compiler needs to statically resolve the path from a binary entrypoint (e.g. `main()`, `init()`) to the annotated region — so that it can instrument the call site(s) and wire in the dispatch decision. We call this the **activation path**.

**Your task.** For the candidate below, find the **minimal static path** from a binary entrypoint to the region root. "Minimal" means: the fewest intermediate steps that a static analysis would need to resolve in order to connect the entrypoint to the region. You are producing evidence for what a compiler algorithm would need to handle — not documenting the runtime call stack.

**Candidate:**
- **ID:** ${CANDIDATE_ID}
- **Name:** ${CANDIDATE_NAME}
- **Region root:** `${REGION_ROOT}` in source tree `evaluation/${PROJECT_NAME}/`

**What to produce.** A narrative trace: a sequence of steps from entrypoint to region root. For each step, identify:

1. **From → To**: the source location (file:line) of the call/registration site and what it reaches.
2. **Edge type**: a short label describing what kind of static-analysis resolution this step requires. Invent descriptive labels — do not use codebase-specific names. Think in terms of what a compiler would need to do to resolve this edge statically.

**Guidelines for edge types.** Each edge type you name should represent a category of resolution that is:
- **Statically resolvable in principle** — a compiler with access to the full source tree and type information could resolve it without running the program. Dynamic dispatch through `reflect.Call`, user-supplied plugin paths, or runtime-determined function pointers are NOT statically resolvable; note these as boundaries if you encounter them.
- **Codebase-agnostic** — the label should describe the *language-level or pattern-level mechanism*, not the framework. "interface-method-dispatch" is good; "caddy-module-provision" is not. If a framework uses a general mechanism (e.g. a struct-field function value set during init), name the general mechanism.
- **Granular enough to be distinct** — "direct-function-call" and "method-call-on-concrete-type" are worth distinguishing from "interface-method-dispatch" because they require different static-analysis strategies.

Some examples of the kind of labels you might use (not exhaustive — invent your own as needed):
- `direct-function-call` — static, fully resolved at compile time
- `method-call-on-concrete-type` — receiver type is known statically
- `interface-method-dispatch` — caller holds an interface; implementation resolved by type analysis
- `function-value-in-struct-field` — a func stored in a struct field, set at init/config time
- `goroutine-launch` — `go f()` or `go func() { ... }()`
- `closure-capture` — a closure binds over a function/method and is passed or stored
- `channel-send-receive` — value (including a func or a work item) passed through a channel
- `http-handler-registration` — `mux.Handle("/path", handler)` or equivalent
- `callback-registration` — a function is stored to be called later (event hook, observer, etc.)
- `init-function` — Go `init()` called implicitly at package load
- `struct-literal-field-assignment` — a func/interface value assigned in a struct literal

**Trace format:**

```
## Activation path for ${CANDIDATE_ID}

### Entrypoint

<file:line> — what this entrypoint does (e.g. "main() starts the HTTP server").

### Step 1: <short description>

- **From:** `<file:line>` — <what happens here>
- **To:** `<file:line>` — <what is reached>
- **Edge type:** `<your-label>`
- **Resolution notes:** <1–2 sentences: what would a static analyzer need to know to resolve this edge? e.g. "needs whole-program type analysis to resolve the Issuer interface to InternalIssuer">

### Step 2: ...

[continue until you reach the region root]

### Region root reached

`<file:line>` — <function/method name>

### Summary

- **Path length:** <N steps>
- **Edge types encountered:** <comma-separated list>
- **Hardest edge:** <which step is the most difficult for static analysis, and why>
- **Static-analysis feasibility:** <one sentence: could a compiler resolve this entire path with type analysis + call graph, or is there a dynamic gap?>
```

**Constraints:**
1. Every file:line you cite must be one you actually read. No speculation.
2. Find the **shortest** path. If multiple paths exist (e.g. the region is reachable from both an HTTP handler and a cron job), pick the shortest and mention the alternatives briefly in the summary.
3. Do not invent framework-specific edge type labels. If a framework mechanism is just a wrapper around a general Go pattern, name the Go pattern.
4. If you hit a dynamic boundary (reflect, plugin, runtime function pointer) that is truly not statically resolvable, say so explicitly and stop the trace there. Note what is on the other side.

**Output destination:** Write your final answer to `${OUTPUT_PATH}`. Do not write anywhere else.

**Tools available.** Standard read/grep/find. You may run `go list`, `grep`, etc. against the source tree at `evaluation/${PROJECT_NAME}/`. Do not modify the source tree.
