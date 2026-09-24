FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
# modernc.org/sqlite is pure Go, so no cgo and no C toolchain is needed.
RUN CGO_ENABLED=0 go build -o /out/url-shortener .

FROM alpine:3.20

RUN adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/url-shortener /app/url-shortener
RUN mkdir -p /app/data && chown -R app /app/data
USER app

ENV ADDR=:8080 \
    BASE_URL=http://localhost:8080 \
    DB_PATH=/app/data/urls.db

EXPOSE 8080
CMD ["/app/url-shortener"]
