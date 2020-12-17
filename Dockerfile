FROM golang:1.15-alpine as build
RUN apk update && apk add build-base
WORKDIR /src
COPY . .
RUN make

FROM alpine:3.12
COPY --from=build /src/bin/* /usr/local/bin/
ENTRYPOINT ["bqmetricsd"]