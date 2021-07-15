#!/bin/bash

VERSION="v${VERSION:-"0.1.0"}"
CSV_NAME="metallb-operator.${VERSION}"
NAMESPACE=${NAMESPACE:-"metallb-system"}

ATTEMPTS=0
MAX_ATTEMPTS=60
csv_created=false
until $csv_created || [ $ATTEMPTS -eq $MAX_ATTEMPTS ]
do
    echo "waiting for csv to be created attempt:${ATTEMPTS}"
    if kubectl get csv -n $NAMESPACE $CSV_NAME; then
        echo "csv created!"
        csv_created=true
    else    
        echo "failed, retrying"
        sleep 5
    fi
    (( ATTEMPTS++ ))
done

if ! $csv_created; then 
    echo "Timed out waiting for csv to be created"
    exit 1
fi

ATTEMPTS=0
MAX_ATTEMPTS=60
csv_succeeded=false
until $csv_succeeded || [ $ATTEMPTS -eq $MAX_ATTEMPTS ]
do
    echo "waiting for csv to be in phase Succeeded attempt:${ATTEMPTS}"
    if [[ $(kubectl get csv -n $NAMESPACE $CSV_NAME -o 'jsonpath={.status.phase}') == "Succeeded" ]]; then
        echo "csv phase Succeeded"
        csv_succeeded=true
    else    
        echo "failed, retrying"
        sleep 5
    fi
    (( ATTEMPTS++ ))
done

if ! $csv_succeeded; then 
    echo "Timed out waiting for csv to be in Succeeded phase"
    exit 1
fi
