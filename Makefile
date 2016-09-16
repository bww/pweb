
# the product we're building
PRODUCT := pweb
# the product's main package
MAIN := ./src/main
# fix our gopath
GOPATH := $(GOPATH):$(PWD)

# build and packaging
TARGETS		:= $(PWD)/target
BUILD_DIR	:= $(TARGETS)/$(PRODUCT)
PRODUCT		:= $(BUILD_DIR)/bin/$(PRODUCT)

# sources
SRC  = $(shell find src -name \*.go -print)
SKEL = $(BUILD_DIR)/bin

.PHONY: all build test clean

all: build

$(SKEL):
	mkdir -p $@

$(PRODUCT): $(SKEL) $(SRC)
	go build -o $@ $(MAIN)

build: $(PRODUCT) ## Build the service

test: ## Run tests
	go test -test.v hunit

clean: ## Delete the built product and any generated files
	rm -rf $(TARGETS)
