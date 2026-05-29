# syntax=docker/dockerfile:1
# Multi-stage build shared by both binaries. Build a specific service with
#   docker build --build-arg SVC=api    .
#   docker build --build-arg SVC=worker .
FROM golang:1.26-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG SVC=api
RUN CGO_ENABLED=0 go build -trimpath -o /out/app ./cmd/${SVC}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates \
  && adduser -D -u 10001 appuser
USER appuser
COPY --from=build /out/app /app/app
ENTRYPOINT ["/app/app"]
