CONTAINER_REGISTRY ?= docker.io/eirn
IMAGE_NAME ?= $(CONTAINER_REGISTRY)/demo-monolith
IMAGE_TAG  ?= workpool

# Kubernetes manifest files location
K8S_DIR ?= demo/monolith/k8s
K8S_DEPLOYMENT_FILE = $(K8S_DIR)/monolith-deployment.yaml
K8S_SERVICE_FILE    = $(K8S_DIR)/monolith-service.yaml

GENERATED_MANIFEST_DIR ?= output/k8s

.PHONY: demo build push deploy undeploy build-and-deploy-local help

build:
	go build -o monolift ./cmd/main.go

demo: build-demo push-demo deploy-demo

build-demo:
	@echo "--- Building Docker image: $(IMAGE_NAME):$(IMAGE_TAG)"
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) -f demo/monolith/Dockerfile .

push-demo: build-demo
	@echo "--- Pushing Docker image: $(IMAGE_NAME):$(IMAGE_TAG) ---"
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

deploy-demo:
	@echo "--- Deploying to Kubernetes using manifests in $(K8S_DIR) ---"
	kind load docker-image $(IMAGE_NAME):$(IMAGE_TAG) --name kind
	kubectl apply -f $(K8S_DIR)

undeploy:
	@echo "--- Undeploying from Kubernetes ---"
	kubectl delete -f $(K8S_DEPLOYMENT_FILE) --ignore-not-found=true
	kubectl delete -f $(K8S_SERVICE_FILE) --ignore-not-found=true

apply-all: apply-demo apply-generated

apply-demo:
	@echo "--- applying demo/monolith ---"
	kubectl apply -f $(K8S_DIR)

apply-generated:
	@echo "---applying generated output---"
	kubectl apply -f $(GENERATED_MANIFEST_DIR)

delete-all: delete-demo delete-generated

delete-demo:
	kubectl delete -f $(K8S_DIR)

delete-generated:
	kubectl delete -f $(GENERATED_MANIFEST_DIR)

delete-redis:
	@echo "--- deleting redis manifests ---"
	kubectl delete -f $(K8S_DIR)/redis.yaml 
	kubectl wait --for=delete -f $(K8S_DIR)/redis.yaml --timeout=60s || true
	kubectl delete -f $(K8S_DIR)/redis-multi.yaml 
	kubectl wait --for=delete -f $(K8S_DIR)/redis-multi.yaml --timeout=60s || true


reset-redis:
	@echo "---deleting and then recreating redis---"
	kubectl delete -f $(K8S_DIR)/redis.yaml --ignore-not-found=true
	kubectl wait --for=delete -f $(K8S_DIR)/redis.yaml --timeout=60s || true
	kubectl delete -f $(K8S_DIR)/redis-multi.yaml --ignore-not-found=true
	kubectl wait --for=delete -f $(K8S_DIR)/redis-multi.yaml --timeout=60s || true
	kubectl apply -f $(K8S_DIR)/redis.yaml
	kubectl apply -f $(K8S_DIR)/redis-multi.yaml