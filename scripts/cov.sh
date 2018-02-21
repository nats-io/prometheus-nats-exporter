#!/bin/bash -e
# Run from directory above via ./scripts/cov.sh

rm -rf /tmp/cov
mkdir -p /tmp/cov
go test -v -covermode=atomic -coverprofile=/tmp/cov/collector.out ./collector
go test -v -covermode=atomic -coverprofile=/tmp/cov/exporter.out ./exporter
gocovmerge /tmp/cov/*.out > /tmp/acc.out
rm -rf /tmp/cov

# If we have an arg, assume travis run and push to coveralls. Otherwise launch browser results
if [[ -n $1 ]]; then
    $HOME/gopath/bin/goveralls -coverprofile=/tmp/acc.out -service travis-ci
    rm -rf /tmp/acc.out
else
    go tool cover -html=/tmp/acc.out
fi
