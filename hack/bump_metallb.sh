#!/bin/bash

HASH=`git ls-remote https://github.com/metallb/metallb | grep refs/heads/main | cut -f 1`
sed -i "s/export METALLB_COMMIT_ID=.*/export METALLB_COMMIT_ID=\"$HASH\"/g" hack/common.sh
