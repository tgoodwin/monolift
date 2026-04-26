CONTAINER_REGISTRY ?= ghcr.io/tgoodwin
IMAGE_NAME ?= $(CONTAINER_REGISTRY)/demo-monolith
IMAGE_TAG  ?= workpool

# Kubernetes manifest files location
K8S_DIR ?= demo/monolith/k8s
K8S_DEPLOYMENT_FILE = $(K8S_DIR)/monolith-deployment.yaml
K8S_SERVICE_FILE    = $(K8S_DIR)/monolith-service.yaml

GENERATED_MANIFEST_DIR ?= output/k8s

.PHONY: demo build push deploy undeploy build-and-deploy-local help verify-evaluation-untouched

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
	kind load docker-image $(IMAGE_NAME):$(IMAGE_TAG) --name operator-perf-test
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

reset-redis:
	@echo "---deleting and then recreating redis---"
	kubectl delete -f $(K8S_DIR)/redis.yaml
	kubectl wait --for=delete -f $(K8S_DIR)/redis.yaml --timeout=60s || true
	kubectl apply -f $(K8S_DIR)/redis.yaml

include test/e2e/Makefile.include

verify-evaluation-untouched:
	go test ./test/e2e/stubcompiler -run TestCaddySourceTreeUntouched -count=1

# ----------------------------------------------------------------------------
# RSS perf harness (SPRINT-0010).
# ----------------------------------------------------------------------------

MEMCHECK_RUNNER ?= ./test/memcheck/run.sh

MEMCHECK_SHAPE_LABEL ?= baseline-shape
MEMCHECK_SHAPE_ARTIFACT ?= test/memcheck/baseline-shape.json
MEMCHECK_SHAPE_BASELINE ?=
MEMCHECK_SHAPE_TARGET_REDUCTION_PCT ?= 0
MEMCHECK_SHAPE_RECORD_ONLY ?= 1
MEMCHECK_SHAPE_RSS_LIMIT_MB ?= 1536
MEMCHECK_SHAPE_WALL_LIMIT_SEC ?= 180

MEMCHECK_POCKETBASE_LABEL ?= baseline-pocketbase
MEMCHECK_POCKETBASE_ARTIFACT ?= test/memcheck/baseline-pocketbase.json
MEMCHECK_POCKETBASE_BASELINE ?=
MEMCHECK_POCKETBASE_TARGET_REDUCTION_PCT ?= 0
MEMCHECK_POCKETBASE_RECORD_ONLY ?= 1
MEMCHECK_POCKETBASE_RSS_LIMIT_MB ?= 3072
MEMCHECK_POCKETBASE_WALL_LIMIT_SEC ?= 600

MEMCHECK_PKG_LABEL ?= baseline-full
MEMCHECK_PKG_ARTIFACT ?= test/memcheck/baseline-full.json
MEMCHECK_PKG_BASELINE ?=
MEMCHECK_PKG_TARGET_REDUCTION_PCT ?= 0
MEMCHECK_PKG_RECORD_ONLY ?= 1
MEMCHECK_PKG_RSS_LIMIT_MB ?= 4096
MEMCHECK_PKG_WALL_LIMIT_SEC ?= 900
MEMCHECK_PKG_ABSOLUTE_PEAK_LIMIT_MB ?= 3072
MEMCHECK_PKG_STABILITY_LIMIT_PCT ?= 25
MEMCHECK_PKG_GO_TEST_FLAGS ?= -p 1 -parallel 1
MEMCHECK_ACCEPTED_ARTIFACT ?= test/memcheck/after-fix-4.json
MEMCHECK_LATEST_ARTIFACT ?= test/memcheck/latest-memcheck.json
MEMCHECK_SEEDS ?= $(if $(strip $(SEED)),$(SEED),101,202,303)

.PHONY: perf-rss-shape perf-rss-pocketbase perf-rss-pkg memcheck

perf-rss-shape:
	MEMCHECK_SEEDS="$(MEMCHECK_SEEDS)" \
	MEMCHECK_LABEL="$(MEMCHECK_SHAPE_LABEL)" \
	MEMCHECK_OUTPUT="$(MEMCHECK_SHAPE_ARTIFACT)" \
	MEMCHECK_BASELINE="$(MEMCHECK_SHAPE_BASELINE)" \
	MEMCHECK_TARGET_REDUCTION_PCT="$(MEMCHECK_SHAPE_TARGET_REDUCTION_PCT)" \
	MEMCHECK_RECORD_ONLY="$(MEMCHECK_SHAPE_RECORD_ONLY)" \
	$(MEMCHECK_RUNNER) \
		--rss-limit-mb "$(MEMCHECK_SHAPE_RSS_LIMIT_MB)" \
		--wall-limit-sec "$(MEMCHECK_SHAPE_WALL_LIMIT_SEC)" \
		-- \
		go test ./pkg/compiler/shape -count=1 -shuffle=__SEED__

perf-rss-pocketbase:
	MEMCHECK_SEEDS="$(MEMCHECK_SEEDS)" \
	MEMCHECK_LABEL="$(MEMCHECK_POCKETBASE_LABEL)" \
	MEMCHECK_OUTPUT="$(MEMCHECK_POCKETBASE_ARTIFACT)" \
	MEMCHECK_BASELINE="$(MEMCHECK_POCKETBASE_BASELINE)" \
	MEMCHECK_TARGET_REDUCTION_PCT="$(MEMCHECK_POCKETBASE_TARGET_REDUCTION_PCT)" \
	MEMCHECK_RECORD_ONLY="$(MEMCHECK_POCKETBASE_RECORD_ONLY)" \
	$(MEMCHECK_RUNNER) \
		--rss-limit-mb "$(MEMCHECK_POCKETBASE_RSS_LIMIT_MB)" \
		--wall-limit-sec "$(MEMCHECK_POCKETBASE_WALL_LIMIT_SEC)" \
		--env MONOLIFT_CORPUS_TESTS=1 \
		-- \
		go test ./pkg/compiler/extract -run TestAnalyzeDetectsPocketBaseRefusals -count=1 -shuffle=__SEED__

perf-rss-pkg:
	MEMCHECK_SEEDS="$(MEMCHECK_SEEDS)" \
	MEMCHECK_LABEL="$(MEMCHECK_PKG_LABEL)" \
	MEMCHECK_OUTPUT="$(MEMCHECK_PKG_ARTIFACT)" \
	MEMCHECK_BASELINE="$(MEMCHECK_PKG_BASELINE)" \
	MEMCHECK_TARGET_REDUCTION_PCT="$(MEMCHECK_PKG_TARGET_REDUCTION_PCT)" \
	MEMCHECK_RECORD_ONLY="$(MEMCHECK_PKG_RECORD_ONLY)" \
	MEMCHECK_ABSOLUTE_PEAK_LIMIT_MB="$(MEMCHECK_PKG_ABSOLUTE_PEAK_LIMIT_MB)" \
	MEMCHECK_STABILITY_LIMIT_PCT="$(MEMCHECK_PKG_STABILITY_LIMIT_PCT)" \
	$(MEMCHECK_RUNNER) \
		--rss-limit-mb "$(MEMCHECK_PKG_RSS_LIMIT_MB)" \
		--wall-limit-sec "$(MEMCHECK_PKG_WALL_LIMIT_SEC)" \
		--absolute-peak-limit-mb "$(MEMCHECK_PKG_ABSOLUTE_PEAK_LIMIT_MB)" \
		--stability-limit-pct "$(MEMCHECK_PKG_STABILITY_LIMIT_PCT)" \
		-- \
		go test ./pkg/... $(MEMCHECK_PKG_GO_TEST_FLAGS) -count=1 -shuffle=__SEED__

memcheck:
	$(MAKE) perf-rss-pkg \
		MEMCHECK_PKG_LABEL=memcheck \
		MEMCHECK_PKG_ARTIFACT=$(MEMCHECK_LATEST_ARTIFACT) \
		MEMCHECK_PKG_BASELINE=$(MEMCHECK_ACCEPTED_ARTIFACT) \
		MEMCHECK_PKG_TARGET_REDUCTION_PCT=0 \
		MEMCHECK_PKG_RECORD_ONLY=0
	@status=$$(sed -n 's/.*"status": "\(.*\)".*/\1/p' "$(MEMCHECK_LATEST_ARTIFACT)" | head -n 1); \
	if [ "$$status" = "regressed" ] || [ "$$status" = "killed_rss" ] || [ "$$status" = "killed_time" ]; then \
		echo "memcheck failed with status $$status"; \
		exit 1; \
	fi
