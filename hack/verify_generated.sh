#!/bin/bash

if [[ -n "$(git status --porcelain .)" ]]; then
        echo "uncommitted generated files. Please check the differences and commit them."
        echo "$(git status --porcelain .)"
        exit 1
fi
