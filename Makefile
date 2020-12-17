MODULE := $(shell go list -m)
PACKAGES := $(shell go list ./...)
GOLINT := $(shell go list -f {{.Target}} golang.org/x/lint/golint)
REPOSITORY := "ovotech/bigquery-metrics-exporter"
VERSION := $(shell git describe --tags --exact-match 2>/dev/null || git log -1 --pretty='%h')

all: test
	$(MAKE) bin/bqmetrics
	$(MAKE) bin/bqmetricsd

${GOLINT}:
	go get -u golang.org/x/lint/golint

bin/%: cmd/% $(shell find pkg -name '*.go')
	go build -ldflags "-X ${MODULE}/pkg/config.Version=${VERSION}" -o $@ ${MODULE}/$<

build:
	docker build -t bigquery-metrics-exporter .

clean:
	rm -rf bin
	docker image rm bigquery-metrics-exporter:latest ${REPOSITORY}:latest ${REPOSITORY}:${VERSION}

fmt:
	go fmt ${PACKAGES}

lint: ${GOLINT}
	${GOLINT} --set_exit_status ${PACKAGES}

push:
	docker tag bigquery-metrics-exporter:latest ${REPOSITORY}:latest
	docker tag bigquery-metrics-exporter:latest ${REPOSITORY}:${VERSION}
	docker push ${REPOSITORY}:latest
	docker push ${REPOSITORY}:${VERSION}

test:
	go test ${PACKAGES}

.PHONY: all build clean fmt lint push test