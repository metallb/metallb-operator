#!/bin/bash

. $(dirname "$0")/common.sh

VERSION="v1.41.1"

docker run --rm -v $(pwd):/app:z -w /app -e GO111MODULE=on golangci/golangci-lint:${VERSION} \
	golangci-lint run --verbose --print-resources-usage --timeout=15m0s
