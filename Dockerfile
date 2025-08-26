# --- Build Stage ---
FROM --platform=$BUILDPLATFORM golang:1.24-alpine3.22 AS builder

WORKDIR /src/k6

COPY . .

ARG TARGETOS
ARG TARGETARCH

# Required for xk6 and building extensions
RUN apk --no-cache add git=~2

# xk6 for building k6 with extensions
RUN go install go.k6.io/xk6/cmd/xk6@latest

RUN xk6 build --output /usr/bin/k6 \
    --replace go.k6.io/k6=. \
    --with github.com/szkiba/xk6-top@latest \
    --with github.com/LeonAdato/xk6-output-statsd@v0.2.1

# --- Runtime Stage ---
FROM alpine:3.22 AS release

RUN adduser -D -u 12345 -g 12345 k6
COPY --from=builder /usr/bin/k6 /usr/bin/k6

USER 12345
WORKDIR /home/k6

ENTRYPOINT ["k6"]