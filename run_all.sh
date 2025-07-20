#! /bin/bash

MANIFEST_RUN='init'

init_social_graph() {
    python demo/monolith/test/init_social_graph.py --ip $(get_public_ip)  --port 80 --compose
}

run_throughput() {
    go run loadgen.go --ip   $(get_public_ip) --port 80 --output-file $1 --early-exit --workload mixed
}

get_public_ip() {
    # Get the public IP address of the Kubernetes cluster
    if [[ $MANIFEST_RUN == "monolith" ]]; then
        kubectl get svc monolith-service -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
    else 
        kubectl get svc entrypoint-service -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
    fi
}


deploy_shared_manifests() {
    echo "Deploying shared manifests..."
    kubectl apply -f manifests/shared_manifests
    sleep 10
    echo "Shared manifests deployed."
}   


run_step() {
    MANIFEST_RUN=$1
    echo "Running step: $1"
    echo "Resetting Redis..."
    make reset-redis
    sleep 10
    echo "Applying manifests for $1..."
    kubectl apply -f manifests/$1
    sleep 20
    echo "initializing social graph..."
    init_social_graph
    # wait 10 seconds before running the load test
    echo "Waiting for 10 seconds before running the load test..."
    echo "Running load test for $1..."
    # Run the load test
    # python throughput_test.py --ip $(get_public_ip) --port 80 --output results/$1-throughput-01.csv
    run_throughput results/$1/throughput-utah.csv

    echo "Load test for $1 completed."
    echo "Cleaning up resources for $1..."

    kubectl delete -f manifests/$1
    echo "Resources for $1 cleaned up."
}

run_all() {
    echo "Running all steps..."
    deploy_shared_manifests
    run_step full
    run_step monolith
    run_step user
    run_step socialgraph
    run_step timeline
    run_step post
    run_step simple_profile
    run_step profile_guided

    echo "All steps completed."
}

run_all