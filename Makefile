export GOFLAGS := -mod=vendor
export GO111MODULE := on

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
