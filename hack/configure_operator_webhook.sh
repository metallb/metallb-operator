#!/bin/bash

ENABLE_OPERATOR_WEBHOOK="${ENABLE_OPERATOR_WEBHOOK:-true}"

yq e --inplace '.spec.template.spec.containers[0].env[] |= select (.name=="ENABLE_OPERATOR_WEBHOOK").value|="'${ENABLE_OPERATOR_WEBHOOK}'"' config/manager/env.yaml
