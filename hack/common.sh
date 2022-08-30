#!/bin/bash

set -e

pushd .
cd "$(dirname "$0")/.."

function finish {
    popd
}
trap finish EXIT

GOPATH="${GOPATH:-~/go}"
export GOFLAGS="${GOFLAGS:-"-mod=vendor"}"

export PATH=$PATH:$GOPATH/bin

mkdir -p _cache

export METALLB_COMMIT_ID="8b28ef52ec3383cfc6d844ac94a5565a261faa0d"
export METALLB_PATH=_cache/metallb

export METALLB_SC_FILE=$(dirname "$0")/securityContext.yaml

function fetch_metallb() {
    if [[ ! -d "$METALLB_PATH" ]]; then
        curl -L https://github.com/metallb/metallb/tarball/"$METALLB_COMMIT_ID" | tar zx -C _cache
        rm -rf "$METALLB_PATH"
        mv _cache/metallb-metallb-* "$METALLB_PATH"
    fi
}
