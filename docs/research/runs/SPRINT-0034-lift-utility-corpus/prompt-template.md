# Phase 1 prompt template

This is the prompt fed to each Phase 1 agent. Variables in `${...}` are substituted per invocation.

---

You are surveying a Go codebase for **useful lift regions** in support of a research project called Monolift.

**Monolift in one paragraph.** Monolift is a research compiler that takes developer-annotated code regions (today: function or interface-method definitions) and emits two implementations — one in-process, one remote-callable — plus a runtime oracle that decides per call whether to invoke locally or dispatch to a remotely scaled replica based on metrics (CPU, memory, instructions per second). The win condition is when the lifted region has variable, expensive-under-load work and is loosely enough coupled to the rest of the binary that remote dispatch is feasible. See `research/RESEARCH_BRIEF.md` and the PLOS '25 paper at `research/monolift_PLOS.pdf` for context.

**Your task.** Read the source tree at `${PROJECT_PATH}` (project: ${PROJECT_NAME}) and propose **5 to 12** candidate lift regions, ranked from best to most marginal. Score each against the rubric at `docs/research/runs/SPRINT-0034-lift-utility-corpus/rubric.md`. Use the per-candidate output format defined in that rubric exactly.

**Constraints.**

1. Every candidate must cite a specific file:line you actually read. No speculation.
2. Score each candidate against all five rubric criteria. A candidate with any `no` should be excluded or kept only with explicit defense.
3. Do not pick framework infrastructure (the mux, the cron scheduler, the queue dispatcher). Pick the *unit of work* that the framework hands control to.
4. Do not pick regions that hold a long-lived per-request connection (WebSockets, SSE).
5. You may re-pick regions from `docs/research/runs/SPRINT-0033-lift-target-catalog.md` only if they pass the utility rubric on their own merits — justify fresh.
6. Output is one Markdown document. Begin with a 3–5 sentence "Project read" paragraph noting what kind of system this is and where the computationally expensive paths cluster, then list candidates in rank order using the rubric's format.

**Output destination.** Write your final answer to `${OUTPUT_PATH}`. Do not write anywhere else.

**Tools available.** Standard read/grep/find. You may run `go list`, `grep`, etc. against the source tree. Do not modify the source tree. Do not run the project. Do not run tests.

**Time/scope.** Aim for thoroughness over speed; the cost of a missed candidate is higher than the cost of a marginal extra one. Cap at 12 candidates so the cross-review phase stays tractable.

When done, close with a one-paragraph "Honest assessment" noting: which candidates you are most confident about, which ones are genuinely marginal, and any region in this codebase that you suspect is a great lift candidate but couldn't justify because the rubric requires evidence you didn't find.
