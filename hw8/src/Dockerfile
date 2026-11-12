# syntax=docker/dockerfile:1
# ---------- Stage 1: build ----------
# Official Go image (must be new enough for the Go version in go.mod).
# This stage holds the compiler and module cache, but it is NOT shipped.
FROM golang:1.27.0 AS build

# Default directory for all subsequent commands in this stage
WORKDIR /app

# Copy go.mod and go.sum first so the dependency layer is cached
# until these two files change.
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy all Go source files
COPY *.go ./

# Build a static Linux binary.
# CGO_ENABLED=0 removes the dependency on C libraries, so the binary runs on a minimal image.
# GOARCH=amd64 matches the default Fargate (x86_64) platform.
# NOTE: the build now compiles the whole package (".") so everything in the 'src' directory is included.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o hw8-shopping-cart .


# ---------- Stage 2: run ----------
# Minimal runtime image: no shell, no package manager, no Go toolchain.
FROM gcr.io/distroless/static-debian12

# Copy only the compiled binary from the build stage
COPY --from=build /app/hw8-shopping-cart /hw8-shopping-cart

# Optional: document the port the app listens on (the app hardcodes :8080)
# EXPOSE 8080

CMD ["/hw8-shopping-cart"]