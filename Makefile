drepo ?= natsio

prometheus-nats-exporter.docker:
	CGO_ENABLED=0 GOOS=linux go build -o $@ -v -a \
		-tags netgo -tags timetzdata \
		-installsuffix netgo -ldflags "-s -w"

.PHONY: dockerx
dockerx:
	docker buildx bake --load

.PHONY: build
build:
	go build

.PHONY: test
test:
	go test -v -race -count=1 -parallel=1 ./test/...
	go test -v -race -count=1 -parallel=1 ./collector/...
	go test -v -race -count=1 -parallel=1 ./exporter/...

.PHONY: test-cov
test-cov:
	go test -v -race -count=1 -parallel=1 ./test/...
	go test -v -race -count=1 -parallel=1 -coverprofile=collector.out ./collector/...
	go test -v -race -count=1 -parallel=1 -coverprofile=exporter.out ./exporter/...

.PHONY: lint
lint:
	@PATH=$(shell go env GOPATH)/bin:$(PATH)
	@if ! which  golangci-lint >/dev/null; then \
		echo "golangci-lint is required and was not found"; \
		exit 1; \
	fi
	go vet ./...
	$(shell go env GOPATH)/bin/golangci-lint run ./...
