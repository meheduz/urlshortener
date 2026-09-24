# URL Shortener

A minimal URL shortener in Go: POST a long URL, get a short one back, and the
short code 302-redirects to the original. SQLite for storage, no other
dependencies.

```
Long URL → POST /shorten → generate code → store code → original URL
         → return short URL → GET /{code} → 302 redirect
```

## Run with Docker

```bash
docker compose up --build
```

The service listens on http://localhost:8080. SQLite data lives in the
`url-data` Docker volume, so it survives container restarts and rebuilds.

If port 8080 is already taken (`bind: address already in use`), pick another
host port:

```bash
HOST_PORT=8081 docker compose up --build
```

## Run locally

```bash
go run .          # listens on :8080, writes data/urls.db
go test ./...
```

## API

Create a short URL:

```bash
curl -X POST http://localhost:8080/shorten \
  -H 'Content-Type: application/json' \
  -d '{"url":"https://example.com/some/very/long/url"}'
```

```json
{ "short_url": "http://localhost:8080/aB91x" }
```

Follow it:

```bash
curl -i http://localhost:8080/aB91x    # 302, Location: https://example.com/...
```

Unknown codes return `404` with `{"error":"unknown code"}`.

Health check:

```bash
curl http://localhost:8080/health      # {"status":"ok"}
```

## Configuration

| Variable   | Default                  | Purpose                                     |
|------------|--------------------------|---------------------------------------------|
| `ADDR`     | `:8080`                  | Listen address                               |
| `BASE_URL` | `http://localhost:8080`  | Prefix used when building the returned link  |
| `DB_PATH`  | `data/urls.db`           | SQLite file path                             |
| `HOST_PORT`| `8080`                   | Host port compose publishes (compose only)   |

## Notes

- Short codes are 5 random characters from `[a-zA-Z0-9]`; inserts retry on the
  rare primary-key collision.
- Only `http` and `https` URLs with a host are accepted.
- Each request creates a new code, so the same URL may map to several codes.
