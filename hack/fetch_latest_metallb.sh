#!/bin/bash

last_tag=$(git -c 'versionsort.suffix=-' \
    ls-remote --exit-code --refs --sort='version:refname' --tags https://github.com/metallb/metallb.git '*.*.*' \
    | tail --lines=1 \
    | cut --delimiter='/' --fields=3)

echo "$last_tag" > hack/metallb_version.txt
