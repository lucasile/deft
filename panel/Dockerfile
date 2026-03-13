# Stage 1: Build the SvelteKit frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /app
COPY internal/panel/ui/web/package.json internal/panel/ui/web/pnpm-lock.yaml ./
RUN npm install -g pnpm && pnpm install
COPY internal/panel/ui/web ./
RUN pnpm run build

# Stage 2: Build the Go backend
FROM golang:1.26-alpine AS backend-builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# Copy the built frontend from Stage 1 into the correct location for embedding
COPY --from=frontend-builder /app/build ./internal/panel/ui/web/build
RUN go build -o deft-panel ./cmd/deft-panel

# Stage 3: Final minimal image
FROM alpine:latest
WORKDIR /app
COPY --from=backend-builder /app/deft-panel /usr/local/bin/deft-panel
# Default ports: 3000 (UI), 50051 (gRPC)
EXPOSE 3000 50051
CMD ["/usr/local/bin/deft-panel"]
