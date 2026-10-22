.PHONY: all help build run test demo clean fmt \
        podman-build pod-deploy pod-down pod-create pod-run pod-start pod-stop pod-restart pod-rm pod-logs pod-ps pod-health \
        deploy undeploy

# Configuration
BINARY ?= bearabull
IMAGE_NAME ?= bearabull
IMAGE_TAG ?= latest
POD_NAME ?= bearabull-pod
CONTAINER_NAME ?= bearabull-app
PORT ?= 8080
VOLUME_NAME ?= bearabull_data
PODMAN ?= podman
KUBE_MANIFEST ?= deploy/pod.yaml

all: help

help:
	@echo "======================================================================"
	@echo "  Bearabull — Build & Podman Deployment Automation"
	@echo "======================================================================"
	@echo "Local Native Targets:"
	@echo "  make build         - Compile local native binary"
	@echo "  make run           - Run local server"
	@echo "  make demo          - Run local server with demo baseline data"
	@echo "  make test          - Run all Go package tests"
	@echo "  make fmt           - Format Go source code"
	@echo "  make clean         - Clean local binaries and SQLite temp files"
	@echo ""
	@echo "Podman Image Targets:"
	@echo "  make podman-build  - Build container image using Podman"
	@echo ""
	@echo "Podman Pod Deployment Targets:"
	@echo "  make pod-deploy    - Declaratively deploy pod via podman play kube (default deploy)"
	@echo "  make pod-down      - Tear down declarative pod deployment"
	@echo "  make pod-create    - Create native Podman pod with port $(PORT):8080"
	@echo "  make pod-run       - Run container inside native Podman pod"
	@echo "  make pod-start     - Start stopped Podman pod"
	@echo "  make pod-stop      - Stop running Podman pod"
	@echo "  make pod-restart   - Restart Podman pod"
	@echo "  make pod-rm        - Remove Podman pod and its containers"
	@echo "  make pod-logs      - Tail live logs of container inside pod"
	@echo "  make pod-ps        - View Podman pod and container status"
	@echo "  make pod-health    - Test health endpoint on http://localhost:$(PORT)/api/v1/health"
	@echo "  make deploy        - Alias for pod-deploy"
	@echo "  make undeploy      - Alias for pod-down"
	@echo "======================================================================"

# -------------------------------------------------------------
# Local Go targets
# -------------------------------------------------------------
build:
	go build -o $(BINARY) ./cmd/bearabull

run: build
	./$(BINARY)

demo: build
	./$(BINARY) -demo -sync

test:
	go test -v ./...

fmt:
	gofmt -s -w .

clean:
	rm -f $(BINARY)
	rm -f bearabull.db*
	rm -f test_*.db*

# -------------------------------------------------------------
# Podman Image Build
# -------------------------------------------------------------
podman-build:
	@echo "Building Podman image $(IMAGE_NAME):$(IMAGE_TAG)..."
	$(PODMAN) build -t $(IMAGE_NAME):$(IMAGE_TAG) -f Containerfile .

# -------------------------------------------------------------
# Declarative Pod Deployment (podman play kube)
# -------------------------------------------------------------
pod-deploy: podman-build
	@echo "Deploying Podman Pod using $(KUBE_MANIFEST)..."
	@$(PODMAN) play kube --down $(KUBE_MANIFEST) 2>/dev/null || true
	$(PODMAN) play kube $(KUBE_MANIFEST)
	@echo "Pod successfully deployed!"
	@echo "Web Terminal: http://localhost:$(PORT)"
	@echo "Health API:   http://localhost:$(PORT)/api/v1/health"

pod-down:
	@echo "Tearing down Podman Pod ($(KUBE_MANIFEST))..."
	@$(PODMAN) play kube --down $(KUBE_MANIFEST) 2>/dev/null || true

deploy: pod-deploy
undeploy: pod-down

# -------------------------------------------------------------
# Native Imperative Podman Pod Lifecycle
# -------------------------------------------------------------
pod-create:
	@echo "Creating Podman pod $(POD_NAME)..."
	@$(PODMAN) pod rm -f $(POD_NAME) 2>/dev/null || true
	$(PODMAN) pod create --name $(POD_NAME) -p $(PORT):8080

pod-run: podman-build
	@echo "Running $(CONTAINER_NAME) inside pod $(POD_NAME)..."
	@$(PODMAN) pod exists $(POD_NAME) || $(MAKE) pod-create
	@$(PODMAN) rm -f $(CONTAINER_NAME) 2>/dev/null || true
	$(PODMAN) run -d \
		--pod $(POD_NAME) \
		--name $(CONTAINER_NAME) \
		-v $(VOLUME_NAME):/app/data:Z \
		-e APP_PORT=8080 \
		-e APP_ENV=production \
		-e DATABASE_PATH=/app/data/bearabull.db \
		-e YAHOO_FINANCE_ENABLED=true \
		-e ANALYSIS_TIMEZONE=Asia/Kolkata \
		-e TZ=Asia/Kolkata \
		$(IMAGE_NAME):$(IMAGE_TAG)
	@echo "Container running in pod $(POD_NAME)."
	@echo "Web Terminal: http://localhost:$(PORT)"

pod-start:
	$(PODMAN) pod start $(POD_NAME)

pod-stop:
	$(PODMAN) pod stop $(POD_NAME)

pod-restart:
	$(PODMAN) pod restart $(POD_NAME)

pod-rm:
	@$(PODMAN) pod rm -f $(POD_NAME) 2>/dev/null || true

pod-logs:
	$(PODMAN) logs -f $(CONTAINER_NAME)

pod-ps:
	@echo "=== Pod Status ==="
	@$(PODMAN) pod ps --filter name=$(POD_NAME)
	@echo ""
	@echo "=== Pod Containers ==="
	@$(PODMAN) ps --filter pod=$(POD_NAME)

pod-health:
	@curl -s http://localhost:$(PORT)/api/v1/health || echo "Endpoint unreachable"
