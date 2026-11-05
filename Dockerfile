FROM node:20-bookworm-slim AS frontend-build

WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.23-bookworm AS backend-build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=frontend-build /src/frontend/dist ./frontend/dist
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/diagnostico-escolar .

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