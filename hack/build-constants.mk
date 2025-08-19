# Copyright 2022 The KCP Authors.
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

# build-constants.mk: Centralized build constants for KCP

# Go build configuration
GO_BUILD_TAGS ?=
GO_BUILD_FLAGS ?= -v
GO_BUILD_GCFLAGS ?=
GO_BUILD_ASMFLAGS ?=

# Environment-specific optimizations
ifeq ($(KCP_BUILD_ENV),ci)
    GO_BUILD_FLAGS += -race
endif

# Debug build configuration
ifdef DEBUG
    GO_BUILD_GCFLAGS += -N -l
    BUILD_OPTIMIZATION = 
else
    BUILD_OPTIMIZATION ?= -trimpath
endif

# Performance optimization flags
ifdef OPTIMIZE
    GO_BUILD_FLAGS += -a
    GO_BUILD_GCFLAGS += -c=4
endif

# Build caching configuration
ifdef NO_CACHE
    GO_BUILD_FLAGS += -a
endif

# Cross-compilation helpers
define build_binary
	@echo "Building $(1) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(dir $(2))
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build \
		$(GO_BUILD_FLAGS) $(BUILD_OPTIMIZATION) $(EXTRA_BUILD_FLAGS) \
		$(if $(GO_BUILD_TAGS),-tags="$(GO_BUILD_TAGS)") \
		$(if $(GO_BUILD_GCFLAGS),-gcflags="$(GO_BUILD_GCFLAGS)") \
		$(if $(GO_BUILD_ASMFLAGS),-asmflags="$(GO_BUILD_ASMFLAGS)") \
		-ldflags="$(LDFLAGS)" \
		-o $(2) $(1)
endef

# Common build targets list
KCP_BINARIES := \
	kcp \
	virtual-workspaces \
	kcp-front-proxy \
	workload-identity-provider

# Test configuration
TEST_TIMEOUT ?= 30m
TEST_FLAGS ?= -v
TEST_PACKAGES ?= ./...

# Coverage configuration  
COVERAGE_DIR ?= coverage
COVERAGE_PROFILE ?= $(COVERAGE_DIR)/coverage.out
COVERAGE_HTML ?= $(COVERAGE_DIR)/coverage.html

# Build validation
.PHONY: validate-build-env
validate-build-env:
	@echo "Validating build environment..."
	@go version
	@echo "Build flags: $(GO_BUILD_FLAGS)"
	@echo "Optimization: $(BUILD_OPTIMIZATION)"
	@echo "Tags: $(GO_BUILD_TAGS)"
	@echo "Environment: $(KCP_BUILD_ENV)"