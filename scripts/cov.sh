#!/usr/bin/env bash
set -ex

# Run from directory above via ./scripts/cov.sh

if [[ "$OSTYPE" != "linux-gnu" ]]; then
	echo "$OSTYPE detected, skipping upload overage"
	exit 0
fi

if [[ "$(go version)" != *"1.13"* ]]; then
	echo "$(go version) detected, skipping upload overage"
	exit 0
fi

echo "--- creating coverage report ---"

rm -rf /tmp/cov
mkdir -p /tmp/cov
go test -v -covermode=atomic -coverprofile=/tmp/cov/collector.out ./collector
go test -v -covermode=atomic -coverprofile=/tmp/cov/exporter.out ./exporter
$GOPATH/bin/gocovmerge /tmp/cov/*.out > coverage.txt
rm -rf /tmp/cov

# If we have an arg, assume travis run and push to coveralls. Otherwise launch
# browser results
if [[ -n $1 ]]; then
	echo "--- uploading test coverage ---"
	bash <(curl -s https://codecov.io/bash)
else
    go tool cover -html=/tmp/acc.out
fi
