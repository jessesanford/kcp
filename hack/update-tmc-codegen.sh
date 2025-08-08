#!/usr/bin/env bash

# Copyright 2024 The KCP Authors.
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
set -o xtrace

export GOPATH=$(go env GOPATH)

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
pushd "${SCRIPT_ROOT}"
BOILERPLATE_HEADER="$( pwd )/hack/boilerplate/boilerplate.generatego.txt"
popd
CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; go list -f '{{.Dir}}' -m k8s.io/code-generator)}
CLUSTER_CODEGEN_PKG=${CLUSTER_CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; go list -f '{{.Dir}}' -m github.com/kcp-dev/code-generator/v3)}

# Install codegen tools
go install "${CODEGEN_PKG}"/cmd/applyconfiguration-gen
go install "${CODEGEN_PKG}"/cmd/client-gen

source "${CODEGEN_PKG}/kube_codegen.sh"
source "${CLUSTER_CODEGEN_PKG}/cluster_codegen.sh"

# Create directories for TMC clients
mkdir -p ${SCRIPT_ROOT}/pkg/client/tmc/{clientset,applyconfiguration,listers,informers}

# Generate apply configurations for TMC
"$GOPATH"/bin/applyconfiguration-gen \
  --go-header-file "${BOILERPLATE_HEADER}" \
  --output-pkg github.com/kcp-dev/kcp/pkg/client/tmc/applyconfiguration \
  --output-dir "${SCRIPT_ROOT}/pkg/client/tmc/applyconfiguration" \
  github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1

# Generate standard client for TMC
"$GOPATH"/bin/client-gen \
  --go-header-file "${BOILERPLATE_HEADER}" \
  --output-pkg github.com/kcp-dev/kcp/pkg/client/tmc/clientset \
  --output-dir "${SCRIPT_ROOT}/pkg/client/tmc/clientset" \
  --input github.com/kcp-dev/kcp/pkg/apis/tmc/v1alpha1 \
  --input-base="" \
  --apply-configuration-package=github.com/kcp-dev/kcp/pkg/client/tmc/applyconfiguration \
  --clientset-name versioned

# Generate helpers (deepcopy, etc.)
kube::codegen::gen_helpers \
  --boilerplate "${BOILERPLATE_HEADER}" \
  ./pkg/apis

# Generate cluster-aware clients using KCP's cluster codegen
cd pkg
cluster::codegen::gen_client \
  --boilerplate "${BOILERPLATE_HEADER}" \
  --versioned-clientset-dir client/tmc/clientset/versioned/cluster \
  --versioned-clientset-pkg github.com/kcp-dev/kcp/pkg/client/tmc/clientset/versioned/cluster \
  --listers-dir client/tmc/listers \
  --listers-pkg github.com/kcp-dev/kcp/pkg/client/tmc/listers \
  --informers-dir client/tmc/informers/externalversions \
  --informers-pkg github.com/kcp-dev/kcp/pkg/client/tmc/informers/externalversions \
  --with-watch \
  --single-cluster-versioned-clientset-pkg github.com/kcp-dev/kcp/pkg/client/tmc/clientset/versioned \
  --single-cluster-applyconfigurations-pkg github.com/kcp-dev/kcp/pkg/client/tmc/applyconfiguration \
  apis
cd -