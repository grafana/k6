FROM --platform=$BUILDPLATFORM golang:1.27.1-alpine@sha256:8a5910f31396cd4d89662f56c68b3ae31d374308270a1c3bd96672ee5ed43414 as builder
WORKDIR $GOPATH/src/go.k6.io/k6
COPY . .
ARG TARGETOS TARGETARCH
RUN apk --no-cache add git=~2
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags "-X go.k6.io/k6/v2/internal/build.BuildOrigin=release" -o /usr/bin/k6

# Runtime stage
FROM alpine:3.24.2@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6 as release

LABEL org.opencontainers.image.source=https://github.com/grafana/k6

RUN adduser -D -u 12345 -g 12345 k6
COPY --from=builder /usr/bin/k6 /usr/bin/k6

USER 12345
WORKDIR /home/k6

ENTRYPOINT ["k6"]

# Browser-enabled bundle
FROM release as with-browser

USER root

COPY --from=release /usr/bin/k6 /usr/bin/k6
RUN apk --no-cache add chromium chromium-swiftshader

USER 12345

ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROME_PATH=/usr/lib/chromium/

ENV K6_BROWSER_HEADLESS=true
# no-sandbox chrome arg is required to run chrome browser in
# alpine and avoids the usage of SYS_ADMIN Docker capability
ENV K6_BROWSER_ARGS=no-sandbox

ENTRYPOINT ["k6"]
