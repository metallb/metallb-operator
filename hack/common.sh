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

export METALLB_COMMIT_ID=$(cat hack/metallb_ref.txt)
export METALLB_PATH=_cache/metallb

export METALLB_SC_FILE=$(dirname "$0")/securityContext.yaml

function fetch_metallb() {
    if [[ ! -d "$METALLB_PATH" ]]; then
        curl -L https://github.com/metallb/metallb/tarball/"$METALLB_COMMIT_ID" | tar zx -C _cache
        rm -rf "$METALLB_PATH"
        mv _cache/metallb-metallb-* "$METALLB_PATH"
    fi
}

export FRRK8S_PATH=_cache/frr-k8s
function fetch_frrk8s() { # first arg is frr-k8s version, corresponding to metallb's chart dependency
    if [[ ! -d "$FRRK8S_PATH" ]]; then
        curl -L https://github.com/metallb/frr-k8s/tarball/"$1" | tar zx -C _cache
        rm -rf "$FRRK8S_PATH"
        mv _cache/metallb-frr-k8s-* "$FRRK8S_PATH"
    fi
}
