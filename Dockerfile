FROM golang:1.24-alpine3.20 AS builder

# Install all build dependencies in one layer
RUN apk add --no-cache git make build-base

WORKDIR /go/src/github.com/buildwithgrove/path-external-auth-server

# Copy and download dependencies first to leverage caching
COPY go.mod go.sum ./
RUN go mod download

# Copy rest of the code
COPY . .

# Build the application
RUN go build -o /go/bin/auth-server .

FROM alpine:3.19
WORKDIR /app

ARG IMAGE_TAG
ENV IMAGE_TAG=${IMAGE_TAG}

COPY --from=builder /go/bin/auth-server ./

CMD ["./auth-server"]
