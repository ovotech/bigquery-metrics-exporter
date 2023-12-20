MODULE := $(shell go list -m)
PACKAGES := $(shell go list ./...)
GOLINT := bin/golint
GORELEASER := bin/goreleaser
VERSION := $(shell git describe --tags --exact-match 2>/dev/null || git log -1 --pretty='%h')

all: test
	$(MAKE) bin/bqmetrics
	$(MAKE) bin/bqmetricsd

${GOLINT}:
	GOBIN="$(CURDIR)/bin" go install golang.org/x/lint/golint@latest

${GORELEASER}:
	GOBIN="$(CURDIR)/bin" go install github.com/goreleaser/goreleaser@latest

bin/%: cmd/% $(shell find pkg -name '*.go')
	go build -ldflags "-X ${MODULE}/pkg/config.Version=${VERSION}" -o $@ ${MODULE}/$<

build: ${GORELEASER}
	${GORELEASER} release --clean --snapshot

clean:
	rm -rf bin dist

fmt:
	go fmt ${PACKAGES}

lint: ${GOLINT}
	${GOLINT} --set_exit_status ${PACKAGES}

test:
	go test --race ${PACKAGES}

.PHONY: all build clean fmt lint push test