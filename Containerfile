# NOTE: For production deployments, pin base images by digest
# e.g., golang:1.25@sha256:<digest> to ensure reproducible builds.

# Stage 1: Build
FROM golang:1.25 AS builder

ARG TARGETARCH=amd64

WORKDIR /workspace

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Ensure knowledge and experiments directories exist even if empty
RUN mkdir -p /workspace/knowledge /workspace/experiments

ARG VERSION=dev

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags "-X github.com/opendatahub-io/operator-chaos/internal/cli.Version=${VERSION}" \
    -o /operator-chaos ./cmd/operator-chaos

# NOTE: For production deployments, pin base images by digest
# e.g., gcr.io/distroless/static:nonroot@sha256:<digest>

# Stage 2: Runtime
FROM gcr.io/distroless/static:nonroot

COPY --from=builder /operator-chaos /operator-chaos
COPY --from=builder /workspace/knowledge/ /knowledge/
COPY --from=builder /workspace/experiments/ /experiments/

USER 65532:65532

ENTRYPOINT ["/operator-chaos"]
