FROM golang:1.26-alpine AS server
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=server /out/server /app/server
ENV ADDR=:7500
EXPOSE 7500
ENTRYPOINT ["/app/server"]
