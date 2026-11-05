APP ?= media-dl
.DEFAULT_GOAL := build
PORT ?= 8080
WEB_PORT ?= 3000
AIR_VERSION := v1.61.7
TAG ?= $(shell git describe --tags --always)
# 腾讯云容器镜像服务：ccr.ccs.tencentyun.com；可通过 HEZEBIN_DOCKER_REGISTRY 覆盖
HEZEBIN_DOCKER_REGISTRY ?= ccr.ccs.tencentyun.com
DOCKER_REGISTRY ?= $(HEZEBIN_DOCKER_REGISTRY)
DOCKER_NAMESPACE ?= hezebin
IMAGE_REPOSITORY ?= $(DOCKER_REGISTRY)/$(DOCKER_NAMESPACE)/$(APP)
IMAGE ?= $(IMAGE_REPOSITORY):$(TAG)
DOCKER_PLATFORM ?= linux/amd64
PACKAGE_GOOS = $(word 1,$(subst /, ,$(DOCKER_PLATFORM)))
PACKAGE_GOARCH = $(word 2,$(subst /, ,$(DOCKER_PLATFORM)))
PACKAGE_GOARM = $(patsubst v%,%,$(word 3,$(subst /, ,$(DOCKER_PLATFORM))))
DOCKER_USER ?= $(HEZEBIN_DOCKER_USER)
DOCKER_PWD ?= $(HEZEBIN_DOCKER_PWD)
export DOCKER_USER DOCKER_PWD AIR_VERSION

.PHONY: dev build web-build server webui test package package-build package-local login docker-build docker-up docker-down

dev:
	@MEDIA_DL_PORT="$(PORT)" MEDIA_DL_WEB_PORT="$(WEB_PORT)" bash scripts/dev.sh

build: web-build
	CGO_ENABLED=0 go build -o bin/$(APP) .

web-build:
	@test -x webui/node_modules/.bin/vite -a -x webui/node_modules/.bin/tsc || (cd webui && yarn install --frozen-lockfile)
	cd webui && yarn build

server:
	go run . server --port $(PORT) --web-dir ./webui/dist

webui:
	cd webui && yarn dev

test:
	go test ./...
	cd webui && yarn lint && yarn build

package: package-local
	$(MAKE) login
	@echo "==> docker push $(IMAGE)"
	docker push $(IMAGE)
	@echo "==> packaged $(IMAGE)"

package-build: web-build
	@test "$(PACKAGE_GOOS)" = linux || (echo "DOCKER_PLATFORM must target Linux" >&2; exit 1)
	mkdir -p .package/image
	CGO_ENABLED=0 GOOS=$(PACKAGE_GOOS) GOARCH=$(PACKAGE_GOARCH) GOARM=$(PACKAGE_GOARM) go build -trimpath -ldflags='-s -w' -o .package/image/media-dl .

package-local: package-build
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
	$(MAKE) package-local TAG=local IMAGE=$(APP):local

docker-up: docker-build
	DOCKER_PLATFORM=$(DOCKER_PLATFORM) docker compose -f docker-compose.local.yml up --no-build

docker-down:
	docker compose -f docker-compose.local.yml down
