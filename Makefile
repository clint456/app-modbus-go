.PHONY: build test unittest lint clean docker vendor tidy

# 架构和平台配置
GOARCH ?= $(shell go env GOARCH)
GOOS ?= linux
DOCKER_PLATFORM ?= linux/arm64

# 版本和镜像配置
VERSION := $(shell cat ./VERSION 2>/dev/null || echo 0.0.0)
GIT_SHA := $(shell git rev-parse HEAD)
REGISTRY := 172.16.19.76:5000
IMAGE := app-demo-go
BUILD_DATE := $(shell date +%Y-%m-%dT%H-%M-%SZ)

# 构建标志
MICROSERVICES := cmd/app-demo-go
GOFLAGS := -ldflags "-s -w" -trimpath -mod=readonly

# 构建目标
build: $(MICROSERVICES)
$(MICROSERVICES):
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -tags "$(ADD_BUILD_TAGS)" $(GOFLAGS) -o $@ ./cmd

# 快捷构建目标
build-arm64:
	@$(MAKE) GOARCH=arm64 build

build-amd64:
	@$(MAKE) GOARCH=amd64 build

# Docker 构建
docker:
	docker buildx build \
		--platform $(DOCKER_PLATFORM) \
		--build-arg ADD_BUILD_TAGS=$(ADD_BUILD_TAGS) \
		--label "build.date=$(BUILD_DATE)" \
		--label "build.version=$(VERSION)" \
		--label "git_sha=$(GIT_SHA)" \
		-t $(REGISTRY)/$(DOCKER_PLATFORM)/$(IMAGE):$(VERSION)-dev \
		--load \
		.

docker-arm64:
	@$(MAKE) DOCKER_PLATFORM=linux/arm64 docker

docker-amd64:
	@$(MAKE) DOCKER_PLATFORM=linux/amd64 docker

clean:
	rm -f $(MICROSERVICES)