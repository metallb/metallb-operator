#!/bin/bash

HASH=`git ls-remote https://github.com/metallb/metallb | grep refs/heads/main | cut -f 1`
echo $HASH > hack/metallb_ref.txt
