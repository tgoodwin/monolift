# Performance analysis

Re-implementing the throughput–latency experiments from the
PLOS '25 workshop paper, this time against the V2 compiler and the
real-world Go monoliths in the corpus rather than the V1 synthetic demo.

## Rapid prototyping
The goal here is to illustrate that the path to scalability via distribution is not a clear one, therefore its valuable to have the ability to re-cut your monolith cheaply (quickly explore the architectural tradeoff space.)

Idea (same as V1 paper, but on real codebases) produce a variety of distributed architectures, produce throughput-latency curves for each.

Could also add a varying workload dimension (i.e. read heavy vs write heavy) to show some kind of heat map.


## Can Monolift beat conventional autoscaling?
In the V1 paper we evaluated against a static microservice. An obvious question from a cloud practitioner would be "how does this compare to just running that microservice with a cloud autoscaler?"

Conventional autoscaling involves adding more replicas of the whole service into an autoscaling pool, and balancing request load among them. Boot-up time can be slow (need to initialize the entire app, its DB connections, etc) and it may take more resources to handle the same load spike than a finer-grained scaling mechanism (i.e. Monolift) would.

The `processImage` example could be good here: by only scaling out the computationally expensive part, we may be able to autoscale with Monolift and show lower tail latencies *and* lower overall CPU / memory consumption needed to handle a workload at a given latency SLA. 

We can run Monolift on k8s vs an service with regular k8s autoscaling, on the same workload.


## How does Monolift handle multiple, competing lifts in response to performance metrics?
This is the un-implemented runtime monitor / lift-state transition system idea from V1. Doesn't need to be the core focus of the problem (really nailing it smells hard, potentially RL-level hard). But we need some strategy for handling this scenario and a straightforward evaluation of it.
