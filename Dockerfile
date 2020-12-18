FROM alpine:3.12
RUN addgroup -S bqmetrics && adduser -S -G bqmetrics bqmetrics
USER bqmetrics
COPY bqmetrics bqmetricsd /usr/local/bin/
ENTRYPOINT ["bqmetricsd"]