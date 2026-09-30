FROM --platform=$BUILDPLATFORM oven/bun:1.3-alpine AS web
WORKDIR /web
COPY frontend/package.json frontend/bun.lock ./
RUN bun install --frozen-lockfile
COPY frontend/ ./
RUN bun run check

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
COPY --from=web /backend/internal/httpapi/mcpapi/app/mcp-app.html internal/httpapi/mcpapi/app/mcp-app.html
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /out/ ./cmd/server ./cmd/worker

FROM gcr.io/distroless/static-debian12:nonroot AS worker
COPY --from=build /out/worker /app/worker
ENTRYPOINT ["/app/worker"]

FROM gcr.io/distroless/static-debian12:nonroot AS server
COPY --from=build /out/server /app/server
COPY --from=web /web/dist/client /app/web
ENV ADDR=:7500 WEB_DIR=/app/web
EXPOSE 7500
ENTRYPOINT ["/app/server"]
