MODULE := $(shell go list -m)
PACKAGES := $(shell go list ./...)

all: test
	$(MAKE) bin/bqmetrics
	$(MAKE) bin/bqmetricsd

bin/%: cmd/% $(shell find pkg -name '*.go')
	go build -o $@ ${MODULE}/$<

clean:
	rm -rf bin

fmt:
	go fmt ${PACKAGES}

test:
	go test ${PACKAGES}

.PHONY: all clean fmt test