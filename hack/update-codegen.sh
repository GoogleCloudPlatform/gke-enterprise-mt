#!/bin/bash

# Copyright 2017 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_ROOT}/.." && pwd)

export GOBIN="${SCRIPT_ROOT}/tools/bin"
export PATH="${GOBIN}:${PATH}"
GOPATH="$(go env GOPATH)"
export GOPATH

echo "Using following variables for code generation:"
echo ""
echo "SCRIPT_ROOT=${SCRIPT_ROOT}"
echo "REPO_ROOT=${REPO_ROOT}"
echo "GOPATH=${GOPATH}"
echo ""

echo "Output files will be generated in appropriate paths inside ${GOPATH}/src"
echo ""
echo "Installing dependencies..."

# Go code dependencies tracked using https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module
mkdir -p "${GOBIN}"
cd "${SCRIPT_ROOT}/tools"
go install "k8s.io/kube-openapi/cmd/openapi-gen" >/dev/null
OPENAPI_PKG="${GOBIN}"

# Non-Go code dependencies (like shell scripts) need to be handled separately.
cd "${GOBIN}"
rm -rf code-generator
git clone https://github.com/kubernetes/code-generator --quiet
cd code-generator
git checkout 9c63990c847dce9e6dca44ad39f7cc4e547bd55f --quiet # https://github.com/kubernetes/code-generator/releases/tag/v0.22.17
CODEGEN_PKG="${PWD}"

echo "Performing code generation for ProviderConfig CRD"
cd "${REPO_ROOT}"
"${CODEGEN_PKG}"/generate-groups.sh \
  "deepcopy,client,informer,lister" \
  github.com/GoogleCloudPlatform/gke-enterprise-mt/pkg/providerconfig/client github.com/GoogleCloudPlatform/gke-enterprise-mt/apis \
  "providerconfig:v1" \
  --go-header-file "${SCRIPT_ROOT}"/boilerplate.go.txt

echo "Generating openapi for ProviderConfig v1"
"${OPENAPI_PKG}"/openapi-gen \
  --output-file zz_generated.openapi.go \
  --output-pkg github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1 \
  --output-dir "${GOPATH}/src/github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1" \
  --go-header-file "${SCRIPT_ROOT}"/boilerplate.go.txt \
  github.com/GoogleCloudPlatform/gke-enterprise-mt/apis/providerconfig/v1