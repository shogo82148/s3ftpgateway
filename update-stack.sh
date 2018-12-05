#!/usr/bin/env bash

set -ux

ROOT=$(cd "$(dirname "$0/")" && pwd)

aws cloudformation --region us-east-1 update-stack \
        --stack-name shogo82148-s3ftpgateway \
        --template-body "file://${ROOT}/cloudformation.yml" \
        --capabilities CAPABILITY_IAM
