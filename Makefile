# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOGENERATE=$(GOCMD) generate
GOGET=$(GOCMD) get
GOINSTALL=$(GOCMD) install
GOLIST=$(GOCMD) list
GOMOD=$(GOCMD) mod
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOTOOL=$(GOCMD) tool
BINARY_NAME=bosun
BINARY_UNIX=$(BINARY_NAME)_unix
BINARY_MAC=$(BINARY_NAME)_mac
BINARY_WIN=$(BINARY_NAME)_win
BOSUN_PACKAGE=bosun.org/cmd/bosun

SCOLLECTOR_BINARY=scollector
SCOLLECTOR_PACKAGE=bosun.org/cmd/scollector
TSDBRELAY_BINARY=tsdbrelay
TSDBRELAY_PACKAGE=bosun.org/cmd/tsdbrelay

SRCS := $(shell find . -name '*.go')
LINTERS := \
	golang.org/x/lint/golint \
	golang.org/x/tools/cmd/goimports \
	github.com/kisielk/errcheck \
	honnef.co/go/tools/cmd/staticcheck

.PHONY: build
build:
	$(GOBUILD) -v bosun.org/...

.PHONY: build-linux
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -v bosun.org/...

.PHONY: build-darwin
build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -v bosun.org/...

.PHONY: build-windows
build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -v bosun.org/...

.PHONY: deps
deps:
	$(GOGET) -d -v ./...
	npm install

.PHONY: updatedeps
updatedeps:
	$(GOGET) -d -v -u -f ./...

.PHONY: testdeps
testdeps:
	$(GOGET) -d -v -t ./...
	$(GOGET) -v $(LINTERS)

.PHONY: updatetestdeps
updatetestdeps:
	$(GOGET) -d -v -t -u -f ./...
	$(GOGET) -u -v $(LINTERS)

.PHONY: install
install: deps
	$(GOINSTALL) ./...

.PHONY: golint
golint:
	golint -set_exit_status ./...

.PHONY: vet
vet:
	$(GOVET) ./...

.PHONY: generate
generate:
	$(GOGENERATE) ./...
	@if [ -n "$$(git diff --exit-code --name-only)" ]; then \
  		echo "The following files are uncommitted changes in the repository:"; \
  		git diff --name-only; \
  		echo "Please commit the files created by go generate."; \
  		echo "This may be a false positive if there were uncommitted files before running this target."; \
  		exit 1; \
  	fi

.PHONY: goimports
goimports:
	goimports -format-only -w ${SRCS}

.PHONY: goimports-check
goimports-check:
	@if [ ! -z "$$(goimports -format-only -l ${SRCS})" ]; then \
      		echo "Found unformatted source files. Please run"; \
      		echo "  make goimports"; \
      		echo "To automatically format your files"; \
      		exit 1; \
      	fi

.PHONY: tidy
tidy:
	$(GOMOD) tidy

.PHONY: tidy-check
tidy-check: tidy
	@if [ -n "$$(git diff-index --exit-code --ignore-submodules --name-only HEAD | grep -E '^go.(mod|sum)$$')" ]; then \
		echo "go.mod or go.sum has changed after running go mod tidy for you."; \
		echo "Please make sure you review and commit the changes."; \
		exit 1; \
	fi

.PHONY: errcheck
errcheck:
	errcheck ./...

.PHONY: staticcheck
staticcheck:
	staticcheck ./...

.PHONY: test
test:
	$(GOTEST) -v ./...

.PHONY: test-coverprofile
test-coverprofile:
	$(GOTEST) -covermode=count -coverprofile=coverage.out $$($(GOLIST) ./... | grep -v integration) -json > test-report.out

.PHONY: coverage
coverage:
	$(GOTEST) -covermode=count -coverprofile=coverage.out  $$($(GOLIST) ./... | grep -v integration)
	$(GOTOOL) cover -html=coverage.out

.PHONY: checks
checks: goimports-check vet generate tidy-check

.PHONY: clean
clean:
	$(GOCLEAN) -v ./...
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)
	rm -f $(BINARY_MAC)
	rm -f $(BINARY_WIN)

.PHONY: run
run: bosun
	./$(BINARY_NAME)

# Cross compilation
all-os: bosun-linux bosun-darwin bosun-windows
bosun:
	$(GOBUILD) -o $(BINARY_NAME) -v $(BOSUN_PACKAGE)
bosun-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v $(BOSUN_PACKAGE)
bosun-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BINARY_MAC) -v $(BOSUN_PACKAGE)
bosun-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BINARY_WIN) -v $(BOSUN_PACKAGE)
scollector:
	$(GOBUILD) -o $(SCOLLECTOR_BINARY) -v $(SCOLLECTOR_PACKAGE)
tsdbrelay:
	$(GOBUILD) -o $(TSDBRELAY_BINARY) -v $(TSDBRELAY_PACKAGE)


.PHONY: all
all: deps testdeps checks build all-os build test bosun scollector tsdbrelay
