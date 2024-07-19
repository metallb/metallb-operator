# syntax=docker/dockerfile:1.2

FROM --platform=$BUILDPLATFORM docker.io/golang:1.22 AS builder
ARG GIT_COMMIT=dev
ARG GIT_BRANCH=dev

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/
COPY bindata/deployment/ bindata/deployment/
COPY .git/ .git/
COPY Makefile Makefile

ARG TARGETARCH
ARG TARGETOS
ARG TARGETPLATFORM

RUN case ${TARGETPLATFORM} in \
    "linux/arm/v6") export VARIANT="6" ;; \
    "linux/arm/v7") export VARIANT="7" ;; \
    *) export VARIANT="" ;; \
    esac

# Cache builds directory for faster rebuild
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GOARM=$VARIANT \
    go build -v -o /build/configmaptocrs \
    -ldflags "-X 'main.build==${GIT_COMMIT}'" \
    -o manager main.go


# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /

COPY LICENSE /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/bindata/deployment /bindata/deployment

LABEL org.opencontainers.image.authors="metallb" \
    org.opencontainers.image.url="https://github.com/metallb/metallb-operator" \
    org.opencontainers.image.documentation="https://metallb.universe.tf" \
    org.opencontainers.image.source="https://github.com/metallb/metallb-operator" \
    org.opencontainers.image.vendor="metallb" \
    org.opencontainers.image.licenses="Apache-2.0" \
    org.opencontainers.image.description="Metallb Operator" \
    org.opencontainers.image.title="metallb operator" \
    org.opencontainers.image.base.name="gcr.io/distroless/static:nonroot"

USER nonroot:nonroot

ENTRYPOINT ["/manager"]
