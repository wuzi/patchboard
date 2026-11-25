# syntax=docker/dockerfile:1

# 1. Angular front-end → internal/web/dist
FROM node:26-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN mkdir -p ../internal/web/dist && npm run build

# 2. Go binary with the front-end embedded (pure Go, no CGO)
FROM golang:1.27-alpine AS go
WORKDIR /src
COPY go.mod go.sum ./
COPY cmd ./cmd
COPY internal ./internal
COPY --from=web /src/internal/web/dist ./internal/web/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/patchboard ./cmd/patchboard && mkdir -p /out/data

# 3. Minimal runtime, non-root (uid 65532); /data holds the database, uploads and secret
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=go /out/patchboard /patchboard
COPY --from=go --chown=65532:65532 /out/data /data
ENV PB_ADDR=:8080 PB_DATA_DIR=/data
EXPOSE 8080
VOLUME /data
ENTRYPOINT ["/patchboard"]
CMD ["serve"]
