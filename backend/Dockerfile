FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /frameflow ./cmd/server
FROM alpine:3.21
RUN apk add --no-cache ffmpeg
RUN addgroup -S frameflow && adduser -S frameflow -G frameflow
WORKDIR /app
COPY --from=build /frameflow /usr/local/bin/frameflow
RUN mkdir -p /app/data && chown -R frameflow:frameflow /app
USER frameflow
EXPOSE 28741
CMD ["frameflow"]
