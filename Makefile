CONTAINER_REGISTRY ?= ghcr.io/tgoodwin
IMAGE_NAME ?= $(CONTAINER_REGISTRY)/demo-monolith
IMAGE_TAG  ?= dapr

# Kubernetes manifest files location
K8S_DIR ?= demo/monolith/k8s
K8S_DEPLOYMENT_FILE = $(K8S_DIR)/monolith-deployment.yaml
K8S_SERVICE_FILE    = $(K8S_DIR)/monolith-service.yaml

.PHONY: demo build push deploy undeploy build-and-deploy-local help

demo: build-demo push-demo deploy-demo

build-demo:
	@echo "--- Building Docker image: $(IMAGE_NAME):$(IMAGE_TAG)"
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) -f demo/monolith/Dockerfile .

push-demo: build-demo
	@echo "--- Pushing Docker image: $(IMAGE_NAME):$(IMAGE_TAG) ---"
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

deploy-demo:
	@echo "--- Deploying to Kubernetes using manifests in $(K8S_DIR) ---"
	kind load docker-image $(IMAGE_NAME):$(IMAGE_TAG) --name operator-perf-test
	kubectl apply -f $(K8S_DIR)

undeploy:
	@echo "--- Undeploying from Kubernetes ---"
	kubectl delete -f $(K8S_DEPLOYMENT_FILE) --ignore-not-found=true
	kubectl delete -f $(K8S_SERVICE_FILE) --ignore-not-found=true
