# Can Monolift support real, production codebases?

Evaluates the compiler pass design: does Monolift's analysis hold up
on production Go monoliths, or only on the synthetic demo the
workshop paper's prototype was built against? The unit of evaluation
here is the compiler's handling of examples across the corpus of evaluation targets, not the runtime behavior of any single lifted service.

This will require some justification of which code regions we chose to pursue within these evaluation targets.

Maybe we evaluate compilation time, memory consumption, etc, to provide some quantitative insight to this question.
