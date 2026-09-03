APP_NAME := chain-reaction
PKG := github.com/ashwnn/chain-reaction
IMAGE ?= ghcr.io/ashwnn/chain-reaction:dev

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

# ---------------------------------------------------------------------------
# Environment automation ()
# ---------------------------------------------------------------------------

.PHONY: env-setup env-teardown env-healthcheck env-load-image image

env-setup: ## Create kind cluster, deploy Kubernetes Goat + Chain Reaction manifests
	bash scripts/setup-goat-env.sh

env-teardown: ## Delete the evaluation kind cluster (with confirmation)
	bash scripts/teardown-goat-env.sh

env-healthcheck: ## Verify the evaluation environment is ready
	bash scripts/healthcheck-goat-env.sh

image: ## Build the Chain Reaction container image used by kind/Kubernetes jobs
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg DATE=$(DATE) \
		-t $(IMAGE) .

env-load-image: image ## Build and load Chain Reaction image into the kind cluster
	@bash -c 'source scripts/goat-env-config.sh && \
		if [ "$${CHAIN_REACTION_IMAGE}" != "$(IMAGE)" ]; then \
			echo "WARNING: Make IMAGE ($(IMAGE)) does not match scripts/goat-env-config.sh ($${CHAIN_REACTION_IMAGE})." >&2; \
		fi && \
		echo "Loading $(IMAGE) into kind cluster $${KIND_CLUSTER_NAME}..." && \
		kind load docker-image --name "$${KIND_CLUSTER_NAME}" "$(IMAGE)"'

# ---------------------------------------------------------------------------
# Deployment lint
# ---------------------------------------------------------------------------

.PHONY: deploy-lint

# Validate Kubernetes manifest syntax without a live cluster.
# Uses kubectl --dry-run=client which performs schema validation locally.
# Requires: kubectl installed.
deploy-lint:
	@echo "=== Linting deploy/job.yaml ===" && \
	kubectl apply -f deploy/job.yaml --dry-run=client -o yaml > /dev/null && \
	echo "  job.yaml: OK" && \
	echo "=== Linting deploy/validate-job.yaml (rendered via envsubst) ===" && \
	CHAIN_REACTION_IMAGE="ghcr.io/ashwnn/chain-reaction:dev" && \
	JOB_NAME="chain-reaction-validate" && \
	NAMESPACE="chain-reaction" && \
	API_ENV_NAME="OPENAI_API_KEY" && \
	SECRET_NAME="chain-reaction-llm" && \
	REMOTE_OUTPUT="/tmp/artifacts" && \
	REMOTE_STDOUT="/tmp/chain-reaction.stdout" && \
	REMOTE_EXITCODE="/tmp/chain-reaction.exitcode" && \
	REMOTE_DONE="/tmp/chain-reaction.done" && \
	REMOTE_RELEASE="/tmp/chain-reaction.release" && \
	VALIDATE_CMD="chain-reaction validate --output /tmp/artifacts" && \
	export CHAIN_REACTION_IMAGE JOB_NAME NAMESPACE API_ENV_NAME SECRET_NAME && \
	export REMOTE_OUTPUT REMOTE_STDOUT REMOTE_EXITCODE REMOTE_DONE REMOTE_RELEASE && \
	export VALIDATE_CMD && \
	envsubst '${CHAIN_REACTION_IMAGE} ${JOB_NAME} ${NAMESPACE} ${API_ENV_NAME} ${SECRET_NAME} ${REMOTE_OUTPUT} ${REMOTE_STDOUT} ${REMOTE_EXITCODE} ${REMOTE_DONE} ${REMOTE_RELEASE} ${VALIDATE_CMD}' < deploy/validate-job.yaml | kubectl apply -f - --dry-run=client -o yaml > /dev/null && \
	echo "  validate-job.yaml: OK (rendered)" && \
	echo "=== Linting deploy/rbac.yaml ===" && \
	kubectl apply -f deploy/rbac.yaml --dry-run=client -o yaml > /dev/null && \
	echo "  rbac.yaml: OK" && \
	echo "=== Linting deploy/serviceaccount.yaml ===" && \
	kubectl apply -f deploy/serviceaccount.yaml --dry-run=client -o yaml > /dev/null && \
	echo "  serviceaccount.yaml: OK" && \
	echo "=== All manifest checks passed ==="

# ---------------------------------------------------------------------------
# Build / test
# ---------------------------------------------------------------------------

.PHONY: build test tidy run package

build:
	go build -ldflags "-X $(PKG)/internal/buildinfo.Version=$(VERSION) -X $(PKG)/internal/buildinfo.Commit=$(COMMIT) -X $(PKG)/internal/buildinfo.Date=$(DATE)" -o bin/$(APP_NAME) ./cmd/chain-reaction

test:
	go test ./...

tidy:
	go mod tidy

run:
	go run ./cmd/chain-reaction scan

package:
	go run ./cmd/chain-reaction package --output submission-package
