MODULE := $(shell go list -m)
PACKAGES := $(shell go list ./...)
GOLINT := $(shell go list -f {{.Target}} golang.org/x/lint/golint)

all: test
	$(MAKE) bin/bqmetrics
	$(MAKE) bin/bqmetricsd

${GOLINT}:
	go get -u golang.org/x/lint/golint

bin/%: cmd/% $(shell find pkg -name '*.go')
	go build -o $@ ${MODULE}/$<

clean:
	rm -rf bin

fmt:
	go fmt ${PACKAGES}

lint: ${GOLINT}
	${GOLINT} --set_exit_status ${PACKAGES}

test:
	go test ${PACKAGES}

.PHONY: all clean fmt lint test