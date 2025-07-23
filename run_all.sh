#! /bin/bash

MANIFEST_RUN='full'
WORKLOAD_TYPE='save'

init_social_graph() {
    python demo/monolith/test/init_social_graph.py --ip $(get_public_ip)  --port 80 --compose
}

run_throughput() {
    if [[ $WORKLOAD_TYPE == "save" ]]; then
        go run loadgen.go --ip   $(get_public_ip) --port 80 --output-file $1 --early-exit --workload save
    else 
        go run loadgen.go --ip   $(get_public_ip) --port 80 --output-file $1 --early-exit --workload mixed
    fi
}

get_public_ip() {
    # Get the public IP address of the Kubernetes cluster
    if [[ $MANIFEST_RUN == "monolith" ]]; then
        kubectl get svc monolith-service -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
    elif [[ $MANIFEST_RUN == "monolith_large" ]]; then
        kubectl get svc monolith-service -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
    elif [[ $MANIFEST_RUN == "monolith_small" ]]; then
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
    run_throughput results-$WORKLOAD_TYPE/$1/throughput-$2.csv 

    echo "Load test for $1 completed."
    echo "Cleaning up resources for $1..."

    kubectl delete -f manifests/$1
    echo "Resources for $1 cleaned up."
}

run_all() {
    echo "Running all steps..."
    deploy_shared_manifests

    WORKLOAD_TYPE='save'
    echo "Running mixed workload..."
    for i in {1..2}; do
        echo "Iteration $i..."
        run_step "full" "$i"
        run_step "monolith" "$i"
        run_step "monolith_large" "$i"
        run_step "monolith_small" "$i"
        run_step "user" "$i"
        run_step "socialgraph" "$i"
        run_step "timeline" "$i"
        run_step "post" "$i"
        run_step "mixed_profile_half_peak" "$i"
        run_step "save_profile_half_peak" "$i"
        run_step "save_profile_peak" "$i"
        done
    echo "All steps completed."

    WORKLOAD_TYPE='mixed'
    echo "Running mixed workload..."

    for i in {1..2}; do
        echo "Iteration $i..."
        run_step "full" "$i"
        run_step "monolith" "$i"
        run_step "monolith_large" "$i"
        run_step "monolith_small" "$i"
        run_step "user" "$i"
        run_step "socialgraph" "$i"
        run_step "timeline" "$i"
        run_step "post" "$i"
        run_step "mixed_profile_half_peak" "$i"
        run_step "save_profile_half_peak" "$i"
        run_step "save_profile_peak" "$i"
    done
}

run_all