# syntax=docker/dockerfile:1.7
FROM node:20-bookworm-slim AS frontend-build

WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm \
    npm ci --prefer-offline --no-audit --no-fund
COPY frontend/ ./
RUN NODE_OPTIONS=--max-old-space-size=768 npm run build

FROM golang:1.23-bookworm AS backend-build

WORKDIR /src
ENV GOMAXPROCS=2
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download
COPY . ./
COPY --from=frontend-build /src/frontend/dist ./frontend/dist
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -p=2 -trimpath -ldflags="-s -w" -o /out/diagnostico-escolar .

FROM debian:bookworm-slim

WORKDIR /app
COPY --from=backend-build /out/diagnostico-escolar /app/diagnostico-escolar
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /dados

ENV PORT=8080 \
    DATA_DIR=/dados \
    TZ=America/Sao_Paulo

VOLUME /dados
EXPOSE 8080

ENTRYPOINT ["/app/diagnostico-escolar"]