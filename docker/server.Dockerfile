FROM golang:1.26-alpine AS build
WORKDIR /src

# Cache
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
ENV CGO_ENABLED=0
RUN go build -o /out/mcp-sandboxd ./cmd/mcp-sandboxd

FROM alpine:3.20 AS runtime
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 app

USER app
WORKDIR /app

COPY --from=build /out/mcp-sandboxd /app/mcp-sandboxd

# Sandbox image for runtime build
COPY --from=build /src/docker/sandbox.Dockerfile /app/docker/sandbox.Dockerfile

EXPOSE 8080
ENTRYPOINT ["/app/mcp-sandboxd"]
