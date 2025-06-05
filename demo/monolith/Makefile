IMAGE_NAME ?= docker.io/tlg2132/demo-monolith
IMAGE_TAG  ?= latest

# Kubernetes manifest files location
K8S_DIR ?= k8s
K8S_DEPLOYMENT_FILE = $(K8S_DIR)/monolith-deployment.yaml
K8S_SERVICE_FILE    = $(K8S_DIR)/monolith-service.yaml

.PHONY: all build push deploy undeploy build-and-deploy-local help

all: build push deploy

build:
	@echo "--- Building Docker image: $(IMAGE_NAME):$(IMAGE_TAG)"
	docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

push: build
	@echo "--- Pushing Docker image: $(IMAGE_NAME):$(IMAGE_TAG) ---"
	docker push $(IMAGE_NAME):$(IMAGE_TAG)

deploy:
	@echo "--- Deploying to Kubernetes using manifests in $(K8S_DIR) ---"
	kind load docker-image $(IMAGE_NAME):$(IMAGE_TAG) --name operator-perf-test
	kubectl apply -f $(K8S_DEPLOYMENT_FILE)
	kubectl apply -f $(K8S_SERVICE_FILE)

undeploy:
	@echo "--- Undeploying from Kubernetes ---"
	kubectl delete -f $(K8S_DEPLOYMENT_FILE) --ignore-not-found=true
	kubectl delete -f $(K8S_SERVICE_FILE) --ignore-not-found=true