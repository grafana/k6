FROM --platform=$BUILDPLATFORM golang:1.24-alpine3.20 as builder
WORKDIR $GOPATH/src/go.k6.io/k6
COPY . .
ARG TARGETOS TARGETARCH
RUN apk --no-cache add git=~2
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -o /usr/bin/k6

# Runtime stage
FROM alpine:3.20 as release

RUN adduser -D -u 12345 -g 12345 k6
COPY --from=builder /usr/bin/k6 /usr/bin/k6

# Copy the man page from the repo (ensure "man/k6.1" exists in your repo)
COPY man/k6.1 /usr/share/man/man1/k6.1

# Set MANPATH so that the man page is discoverable
ENV MANPATH="/usr/share/man"

USER k6
WORKDIR /home/k6

ENTRYPOINT ["k6"]

# Browser-enabled bundle
FROM release as with-browser

USER root

COPY --from=release /usr/bin/k6 /usr/bin/k6
RUN apk --no-cache add chromium-swiftshader

USER k6

ENV CHROME_BIN=/usr/bin/chromium-browser
ENV CHROME_PATH=/usr/lib/chromium/

ENV K6_BROWSER_HEADLESS=true
# no-sandbox chrome arg is required to run chrome browser in
# alpine and avoids the usage of SYS_ADMIN Docker capability
ENV K6_BROWSER_ARGS=no-sandbox

ENTRYPOINT ["k6"]
