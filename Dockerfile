# Stage 1: Build binaries
FROM --platform=$BUILDPLATFORM golang:1.23-alpine as builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=0.0.0
ARG CHANNEL=dev

WORKDIR /app

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download -x

COPY . .

# Build main binary
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath \
    -ldflags="-w -s -X github.com/sirrobot01/scroblarr/pkg/version.Version=${VERSION} -X github.com/sirrobot01/scroblarr/pkg/version.Channel=${CHANNEL}" \
    -o /scroblarr

# Stage 2: Create directory structure
FROM alpine:3.19 as dirsetup
RUN mkdir -p /app/logs && \
    mkdir -p /app/cache && \
    chmod 777 /app/logs && \
    touch /app/logs/scroblarr.log && \
    chmod 666 /app/logs/scroblarr.log

# Stage 3: Final image
FROM gcr.io/distroless/static-debian12:nonroot

LABEL version = "${VERSION}-${CHANNEL}"

LABEL org.opencontainers.image.source = "https://github.com/sirrobot01/scroblarr"
LABEL org.opencontainers.image.title = "scroblarr"
LABEL org.opencontainers.image.authors = "sirrobot01"
LABEL org.opencontainers.image.documentation = "https://github.com/sirrobot01/scroblarr/blob/main/README.md"

# Copy binaries
COPY --from=builder --chown=nonroot:nonroot /scroblarr /usr/bin/scroblarr

# Copy pre-made directory structure
COPY --from=dirsetup --chown=nonroot:nonroot /app /app


# Metadata
ENV LOG_PATH=/app/logs
EXPOSE 8181 8282
VOLUME ["/app"]
USER nonroot:nonroot

CMD ["/usr/bin/scroblarr", "--config", "/app"]