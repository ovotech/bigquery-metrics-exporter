MODULE := $(shell go list -m)
PACKAGES := $(shell go list ./...)
GOLINT := $(shell go list -f {{.Target}} golang.org/x/lint/golint)
GORELEASER := bin/goreleaser
VERSION := $(shell git describe --tags --exact-match 2>/dev/null || git log -1 --pretty='%h')

all: test
	$(MAKE) bin/bqmetrics
	$(MAKE) bin/bqmetricsd

${GOLINT}:
	go get -u golang.org/x/lint/golint

${GORELEASER}:
	curl -sfL https://install.goreleaser.com/github.com/goreleaser/goreleaser.sh | sh

bin/%: cmd/% $(shell find pkg -name '*.go')
	go build -ldflags "-X ${MODULE}/pkg/config.Version=${VERSION}" -o $@ ${MODULE}/$<

build: ${GORELEASER}
	${GORELEASER} release --rm-dist --snapshot

clean:
	rm -rf bin

fmt:
	go fmt ${PACKAGES}

lint: ${GOLINT}
	${GOLINT} --set_exit_status ${PACKAGES}

test:
	go test --race ${PACKAGES}

.PHONY: all build clean fmt lint push test