FROM alpine:latest

EXPOSE 8000

WORKDIR /app
ENTRYPOINT ["/app/reaper"]
CMD ["-config" "/app/config.toml"]

RUN apk add --update ca-certificates
RUN addgroup -g 10001 app && \
    adduser -G app -u 10001 -D -h /app -s /sbin/nologin app

COPY version.json /app/version.json
COPY reaper.exe /app/reaper

USER app
