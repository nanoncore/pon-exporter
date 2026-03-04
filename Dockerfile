FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ARG VERSION=dev
ARG REVISION=unknown
ARG BRANCH=unknown

RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w \
      -X github.com/nanoncore/pon-exporter/internal/version.Version=${VERSION} \
      -X github.com/nanoncore/pon-exporter/internal/version.Revision=${REVISION} \
      -X github.com/nanoncore/pon-exporter/internal/version.Branch=${BRANCH} \
      -X github.com/nanoncore/pon-exporter/internal/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /pon-exporter ./cmd/pon-exporter

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /pon-exporter /usr/local/bin/pon-exporter

EXPOSE 9876
ENTRYPOINT ["pon-exporter"]
