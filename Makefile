MODULE := $(shell go list -m)
PACKAGES := $(shell go list ./...)
GOLINT := $(shell go list -f {{.Target}} golang.org/x/lint/golint)
VERSION := $(shell git describe --tags --exact-match 2>/dev/null || git log -1 --pretty='%h')

all: test
	$(MAKE) bin/bqmetrics
	$(MAKE) bin/bqmetricsd

${GOLINT}:
	go get -u golang.org/x/lint/golint

bin/%: cmd/% $(shell find pkg -name '*.go')
	go build -ldflags "-X ${MODULE}/pkg/config.Version=${VERSION}" -o $@ ${MODULE}/$<

clean:
	rm -rf bin

fmt:
	go fmt ${PACKAGES}

lint: ${GOLINT}
	${GOLINT} --set_exit_status ${PACKAGES}

test:
	go test ${PACKAGES}

.PHONY: all build clean fmt lint push test