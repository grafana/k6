# See https://tech.davis-hansson.com/p/make/
SHELL := bash
.DELETE_ON_ERROR:
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := all
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-print-directory
BIN ?= $(abspath .tmp/bin)
COPYRIGHT_YEARS := 2020-2023
LICENSE_IGNORE := -e /testdata/
# Set to use a different compiler. For example, `GO=go1.18rc1 make test`.
GO ?= go
TOOLS_MOD_DIR := ./internal/tools
UNAME_OS := $(shell uname -s)
UNAME_ARCH := $(shell uname -m)
PATH_SEP ?= ":"

PROTOC_VERSION := $(shell cat ./.protoc_version)
# For release candidates, the download artifact has a dash between "rc" and the number even
# though the version tag does not :(
PROTOC_ARTIFACT_VERSION := $(shell echo $(PROTOC_VERSION) | sed -E 's/-rc([0-9]+)$$/-rc-\1/g')
PROTOC_DIR ?= $(abspath ./internal/testdata/protoc/$(PROTOC_VERSION))
PROTOC := $(PROTOC_DIR)/bin/protoc

LOWER_UNAME_OS := $(shell echo $(UNAME_OS) | tr A-Z a-z)
ifeq ($(LOWER_UNAME_OS),darwin)
	PROTOC_OS := osx
	ifeq ($(UNAME_ARCH),arm64)
		PROTOC_ARCH := aarch_64
	else
		PROTOC_ARCH := x86_64
	endif
else
	PROTOC_OS := $(LOWER_UNAME_OS)
	PROTOC_ARCH := $(UNAME_ARCH)
endif
PROTOC_ARTIFACT_SUFFIX ?= $(PROTOC_OS)-$(PROTOC_ARCH)

.PHONY: help
help: ## Describe useful make targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "%-30s %s\n", $$1, $$2}'

.PHONY: all
all: ## Build, test, and lint (default)
	$(MAKE) test
	$(MAKE) lint

.PHONY: clean
clean: ## Delete intermediate build artifacts
	@# -X only removes untracked files, -d recurses into directories, -f actually removes files/dirs
	git clean -Xdf

.PHONY: test
test: build ## Run unit tests
	$(GO) test -race -cover ./...
	cd internal/benchmarks && SKIP_DOWNLOAD_GOOGLEAPIS=true $(GO) test -race -cover ./...

.PHONY: benchmarks
benchmarks: build ## Run benchmarks
	cd internal/benchmarks && $(GO) test -bench=. -benchmem -v ./...

.PHONY: build
build: generate ## Build all packages
	$(GO) build ./...

.PHONY: install
install: ## Install all binaries
	$(GO) install ./...

.PHONY: lint
lint: $(BIN)/golangci-lint ## Lint Go
	$(GO) vet ./... ./internal/benchmarks/...
	$(BIN)/golangci-lint run
	cd internal/benchmarks && $(BIN)/golangci-lint run

.PHONY: lintfix
lintfix: $(BIN)/golangci-lint ## Automatically fix some lint errors
	$(BIN)/golangci-lint run --fix
	cd internal/benchmarks && $(BIN)/golangci-lint run --fix

.PHONY: generate
generate: $(BIN)/license-header $(BIN)/goyacc test-descriptors ## Regenerate code and licenses
	PATH="$(BIN)$(PATH_SEP)$(PATH)" $(GO) generate ./...
	@# We want to operate on a list of modified and new files, excluding
	@# deleted and ignored files. git-ls-files can't do this alone. comm -23 takes
	@# two files and prints the union, dropping lines common to both (-3) and
	@# those only in the second file (-2). We make one git-ls-files call for
	@# the modified, cached, and new (--others) files, and a second for the
	@# deleted files.
	comm -23 \
		<(git ls-files --cached --modified --others --no-empty-directory --exclude-standard | sort -u | grep -v $(LICENSE_IGNORE) ) \
		<(git ls-files --deleted | sort -u) | \
		xargs $(BIN)/license-header \
			--license-type apache \
			--copyright-holder "Buf Technologies, Inc." \
			--year-range "$(COPYRIGHT_YEARS)"

.PHONY: upgrade
upgrade: ## Upgrade dependencies
	go get -u -t ./... && go mod tidy -v

.PHONY: checkgenerate
checkgenerate:
	@# Used in CI to verify that `make generate` doesn't produce a diff.
	test -z "$$(git status --porcelain | tee /dev/stderr)"

$(BIN)/license-header: internal/tools/go.mod internal/tools/go.sum
	@mkdir -p $(@D)
	cd $(TOOLS_MOD_DIR) && \
		GOWORK=off $(GO) build -o $@ github.com/bufbuild/buf/private/pkg/licenseheader/cmd/license-header

$(BIN)/golangci-lint: internal/tools/go.mod internal/tools/go.sum
	@mkdir -p $(@D)
	cd $(TOOLS_MOD_DIR) && \
		GOWORK=off $(GO) build -o $@ github.com/golangci/golangci-lint/cmd/golangci-lint

$(BIN)/goyacc: internal/tools/go.mod internal/tools/go.sum
	@mkdir -p $(@D)
	cd $(TOOLS_MOD_DIR) && \
		GOWORK=off $(GO) build -o $@ golang.org/x/tools/cmd/goyacc

internal/testdata/protoc/cache/protoc-$(PROTOC_VERSION).zip:
	@mkdir -p $(@D)
	curl -o $@ -fsSL https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/protoc-$(PROTOC_ARTIFACT_VERSION)-$(PROTOC_ARTIFACT_SUFFIX).zip

.PHONY: protoc
protoc: $(PROTOC)

$(PROTOC): internal/testdata/protoc/cache/protoc-$(PROTOC_VERSION).zip
	@mkdir -p $(@D)
	unzip -o -q $< -d $(PROTOC_DIR) && \
	touch $@

internal/testdata/all.protoset: $(PROTOC) $(sort $(wildcard internal/testdata/*.proto))
	cd $(@D) && $(PROTOC) --descriptor_set_out=$(@F) --include_imports -I. $(filter-out protoc,$(^F))

internal/testdata/desc_test_complex.protoset: $(PROTOC) internal/testdata/desc_test_complex.proto
	cd $(@D) && $(PROTOC) --descriptor_set_out=$(@F) --include_imports -I. $(filter-out protoc,$(^F))

internal/testdata/desc_test_defaults.protoset: $(PROTOC) internal/testdata/desc_test_defaults.proto
	cd $(@D) && $(PROTOC) --descriptor_set_out=$(@F) --include_imports -I. $(filter-out protoc,$(^F))

internal/testdata/desc_test_proto3_optional.protoset: $(PROTOC) internal/testdata/desc_test_proto3_optional.proto
	cd $(@D) && $(PROTOC) --descriptor_set_out=$(@F) --include_imports -I. $(filter-out protoc,$(^F))

internal/testdata/descriptor_impl_tests.protoset: $(PROTOC) internal/testdata/desc_test2.proto internal/testdata/desc_test_defaults.proto internal/testdata/desc_test_proto3.proto internal/testdata/desc_test_proto3_optional.proto
	cd $(@D) && $(PROTOC) --descriptor_set_out=$(@F) --include_imports -I. $(filter-out protoc,$(^F))

internal/testdata/editions/all.protoset: $(PROTOC) $(sort $(wildcard internal/testdata/editions/*.proto))
	cd $(@D) && $(PROTOC) --experimental_editions --descriptor_set_out=$(@F) --include_imports -I. $(filter-out protoc,$(^F))

internal/testdata/source_info.protoset: $(PROTOC) internal/testdata/desc_test_options.proto internal/testdata/desc_test_comments.proto internal/testdata/desc_test_complex.proto
	cd $(@D) && $(PROTOC) --descriptor_set_out=$(@F) --include_source_info -I. $(filter-out protoc,$(^F))

internal/testdata/options/test.protoset: $(PROTOC) internal/testdata/options/test.proto
	cd $(@D) && $(PROTOC) --descriptor_set_out=$(@F) -I. $(filter-out protoc,$(^F))

internal/testdata/options/test_proto3.protoset: $(PROTOC) internal/testdata/options/test_proto3.proto
	cd $(@D) && $(PROTOC) --descriptor_set_out=$(@F) -I. $(filter-out protoc,$(^F))

.PHONY: test-descriptors
test-descriptors: internal/testdata/all.protoset
test-descriptors: internal/testdata/desc_test_complex.protoset
test-descriptors: internal/testdata/desc_test_defaults.protoset
test-descriptors: internal/testdata/desc_test_proto3_optional.protoset
test-descriptors: internal/testdata/descriptor_impl_tests.protoset
test-descriptors: internal/testdata/editions/all.protoset
test-descriptors: internal/testdata/source_info.protoset
test-descriptors: internal/testdata/options/test.protoset
test-descriptors: internal/testdata/options/test_proto3.protoset
