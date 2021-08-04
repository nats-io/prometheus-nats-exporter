export GO111MODULE := on

prometheus-nats-exporter.docker:
	CGO_ENABLED=0 GOOS=linux go build -o $@ -v -a
		-tags netgo -tags timetzdata \
		-installsuffix netgo -ldflags "-s -w"

.PHONY: dockerx
dockerx:
ifneq ($(ver),)
	# Ensure 'docker buildx ls' shows correct platforms.
	docker buildx build \
		--tag natsio/prometheus-nats-exporter:$(ver) --tag natsio/prometheus-nats-exporter:latest \
		--platform linux/amd64,linux/arm/v6,linux/arm/v7,linux/arm64/v8 \
		--file docker/linux/Dockerfile \
		--push .
else
	# Missing version, try this.
	# make dockerx ver=1.2.3
	exit 1
endif

.PHONY: build
build:
	go build

.PHONY: test
test:
	go test -race -count=1 -parallel=1 -v ./test/...
	go test -race -count=1 -parallel=1 -v ./collector/...
	go test -race -count=1 -parallel=1 -v ./exporter/...

.PHONY: test-cover
test-cover:
	./scripts/cov.sh

.PHONY: test-cover-ci
test-cover-ci:
	./scripts/cov.sh CI

.PHONY: install-tools
install-tools:
	cd /tmp && go get github.com/wadey/gocovmerge
	cd /tmp && go get github.com/golangci/golangci-lint/cmd/golangci-lint

.PHONY: lint
lint:
	go vet ./...
	$(shell go env GOPATH)/bin/golangci-lint run \
	  --no-config --exclude-use-default=false --max-same-issues=0 \
		--disable errcheck \
		--enable golint \
		--enable stylecheck \
		--enable interfacer \
		--enable unconvert \
		--enable dupl \
		--enable gocyclo \
		--enable gofmt \
		--enable goimports \
		--enable misspell \
		--enable lll \
		--enable unparam \
		--enable nakedret \
		--enable prealloc \
		--enable scopelint \
		--enable gocritic \
		--enable gochecknoinits \
		./...
