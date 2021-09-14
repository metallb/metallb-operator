#!/bin/bash

ENABLE_OPERATOR_WEBHOOK="${ENABLE_OPERATOR_WEBHOOK:-true}"
WEBHOOK_CONFIG_FILE="${WEBHOOK_CONFIG_FILE:-config/default/kustomization.yaml}"
KUSTOMIZE="${KUSTOMIZE:-kustomize}"

HAS_WEBHOOK_CONFIGURATION=$(${KUSTOMIZE} build config/default | grep "ValidatingWebhookConfiguration")

if [ "${ENABLE_OPERATOR_WEBHOOK}" = "true" ]; then
  # if webhook is not configured, add webhook configuration
  if [ -z "${HAS_WEBHOOK_CONFIGURATION}" ]; then
    cat >> "${WEBHOOK_CONFIG_FILE}" << EOF
- ../webhook
- ../certmanager

patchesStrategicMerge:
- manager_webhook_patch.yaml
- webhookcainjection_patch.yaml

# the following config is for teaching kustomize how to do var substitution
vars:
# [CERTMANAGER] To enable cert-manager, uncomment all sections with 'CERTMANAGER' prefix.
- name: CERTIFICATE_NAMESPACE # namespace of the certificate CR
  objref:
    kind: Certificate
    group: cert-manager.io
    version: v1
    name: serving-cert # this name should match the one in certificate.yaml
  fieldref:
    fieldpath: metadata.namespace
- name: CERTIFICATE_NAME
  objref:
    kind: Certificate
    group: cert-manager.io
    version: v1
    name: serving-cert # this name should match the one in certificate.yaml
- name: SERVICE_NAMESPACE # namespace of the service
  objref:
    kind: Service
    version: v1
    name: webhook-service
  fieldref:
    fieldpath: metadata.namespace
- name: SERVICE_NAME
  objref:
    kind: Service
    version: v1
    name: webhook-service
EOF
  fi
else
  # if webhook is configured, remove webhook configuration
  if [ -n "${HAS_WEBHOOK_CONFIGURATION}" ]; then
    webhook_config_first_line=$(grep --text -n "webhook" "${WEBHOOK_CONFIG_FILE}" | cut -f1 -d: | sort -n | head -n1)
    sed -i ''"${webhook_config_first_line}"',$d' "${WEBHOOK_CONFIG_FILE}"
  fi
fi

yq e --inplace '.spec.template.spec.containers[0].env[] |= select (.name=="ENABLE_OPERATOR_WEBHOOK").value|="'${ENABLE_OPERATOR_WEBHOOK}'"' config/manager/env.yaml
