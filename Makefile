VERSION=$(shell git describe --tags --always)


.PHONY: wire
# generate wire
wire:
	cd cmd/delay && wire

.PHONY: generate
generate:
	cd api/proto && go generate

.PHONY: build
# build
build:
	rm -rf bin/ && mkdir -p bin/ && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/ ./...


.PHONY: docker
# docker
IMAGE_NAME ?= delay
docker:
	docker build -t $(IMAGE_NAME):$(VERSION) .
	docker tag $(IMAGE_NAME):$(VERSION) habor.dev.cdlsxd.cn/common/$(IMAGE_NAME):$(VERSION)
	docker push habor.dev.cdlsxd.cn/common/$(IMAGE_NAME):$(VERSION)
