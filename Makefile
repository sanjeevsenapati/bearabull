.PHONY: build run test demo podman-build podman-run podman-stop podman-logs clean fmt

# Configuration
BINARY ?= bearabull
IMAGE_NAME ?= bearabull
IMAGE_TAG ?= latest
CONTAINER_NAME ?= bearabull-terminal
PORT ?= 8080
VOLUME_NAME ?= bearabull_data
PODMAN ?= podman

# Build local executable
build:
	go build -o $(BINARY) ./cmd/bearabull

# Run local development server
run: build
	./$(BINARY)

# Run with demo baseline data
demo: build
	./$(BINARY) -demo -sync

# Run automated tests
test:
	go test -v ./...

# Format source code
fmt:
	gofmt -s -w .

# Build container image with Podman
podman-build:
	$(PODMAN) build -t $(IMAGE_NAME):$(IMAGE_TAG) .

# Run container with Podman (with persistent volume and port mapping)
podman-run: podman-stop
	$(PODMAN) run -d \
		--name $(CONTAINER_NAME) \
		-p $(PORT):8080 \
		-v $(VOLUME_NAME):/app/data:Z \
		-e APP_PORT=8080 \
		-e APP_ENV=production \
		-e DATABASE_PATH=/app/data/bearabull.db \
		-e YAHOO_FINANCE_ENABLED=true \
		-e ANALYSIS_TIMEZONE=Asia/Kolkata \
		$(IMAGE_NAME):$(IMAGE_TAG)
	@echo "Bearabull running with Podman at http://localhost:$(PORT)"

# Stop and remove Podman container
podman-stop:
	@$(PODMAN) stop $(CONTAINER_NAME) 2>/dev/null || true
	@$(PODMAN) rm $(CONTAINER_NAME) 2>/dev/null || true

# Follow container logs
podman-logs:
	$(PODMAN) logs -f $(CONTAINER_NAME)

# Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -f bearabull.db*
	rm -f test_*.db*

