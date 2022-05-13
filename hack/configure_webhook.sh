#!/bin/bash

ENABLE_OPERATOR_WEBHOOK="${ENABLE_WEBHOOK:-true}"

yq e --inplace '.spec.template.spec.containers[0].env[] |= select (.name=="ENABLE_WEBHOOK").value|="'${ENABLE_WEBHOOK}'"' config/manager/env.yaml
