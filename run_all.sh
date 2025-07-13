#! /bin/bash

function init_social_graph () {
    python demo/monolith/test/init_social_graph.py --ip 128.110.223.23 --port 80 --compose
}

function get_public_ip () {
    # Get the public IP address of the Kubernetes cluster
    kubectl get svc entrypoint-service -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
}

function run_step () {
    echo "Running step: $1"
    echo "Resetting Redis..."
    make reset-redis
    sleep 10
    echo "initializing social graph..."
    init_social_graph
    echo "Applying manifests for $1..."
    kubectl apply -f manifests/$1

    # wait 10 seconds before running the load test
    echo "Waiting for 10 seconds before running the load test..."
    sleep 10
    echo "Running load test for $1..."
    # Run the load test
    python throughput_test.py --ip $(get_public_ip) --port 80 --output results/$1-throughput-01.csv

    echo "Load test for $1 completed."
    echo "Cleaning up resources for $1..."

    kubectl delete -f manifests/$1
    echo "Resources for $1 cleaned up."
}

function run_all () {
    echo "Running all steps..."
    run_step full
    run_step user
    run_step socialgraph
    run_step timeline
    run_step post

    echo "All steps completed."
}