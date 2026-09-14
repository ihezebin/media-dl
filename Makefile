APP ?= media-dl
PORT ?= 8080
TAG ?= $(shell git describe --tags --always)
DOCKER_REGISTRY ?= ghcr.io
DOCKER_NAMESPACE ?= ihezebin
IMAGE_REPOSITORY ?= $(DOCKER_REGISTRY)/$(DOCKER_NAMESPACE)/$(APP)
IMAGE ?= $(IMAGE_REPOSITORY):$(TAG)
DOCKER_PLATFORM ?= linux/amd64
DOCKER_USER ?= $(HEZEBIN_DOCKER_USER)
DOCKER_PWD ?= $(HEZEBIN_DOCKER_PWD)
export DOCKER_USER DOCKER_PWD

.PHONY: build web-build server webui test package package-local login docker-build docker-up docker-down

build: web-build
	CGO_ENABLED=0 go build -o bin/$(APP) .

web-build:
	cd webui && yarn install --frozen-lockfile && yarn build

server:
	go run . server --port $(PORT) --web-dir ./webui/dist --output ./downloads

webui:
	cd webui && yarn dev

test:
	go test . ./httpserver ./internal/...
	cd webui && yarn lint && yarn build

package: package-local
	@echo "==> docker push $(IMAGE)"
	docker push $(IMAGE)
	@echo "==> packaged $(IMAGE)"

package-local: login
	@echo "==> docker build $(IMAGE)"
	docker build --pull --platform $(DOCKER_PLATFORM) --build-arg BUILD_TAG=$(TAG) -t $(IMAGE) .

login:
	@if [ -n "$$DOCKER_USER" ] && [ -n "$$DOCKER_PWD" ]; then \
		echo "==> docker login $(DOCKER_REGISTRY)"; \
		printf '%s' "$$DOCKER_PWD" | docker login --username "$$DOCKER_USER" --password-stdin "$(DOCKER_REGISTRY)"; \
	else \
		echo "==> use existing docker login for $(DOCKER_REGISTRY)"; \
	fi

docker-build:
	docker build --build-arg BUILD_TAG=local -t $(APP):local .

docker-up:
	docker compose -f docker-compose.local.yml up --build

docker-down:
	docker compose -f docker-compose.local.yml down
