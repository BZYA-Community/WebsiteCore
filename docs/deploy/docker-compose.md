# Full Docker Compose Deployment (Alternative)

The recommended production layout runs the binary natively with only the dependencies in Docker ([production.md](production.md)). This page describes the fully containerized alternative: application, database, cache, and search all managed by Compose.

> The repository does not currently ship a Dockerfile. The one below is a starting point — keep it in your deployment repo (or contribute it back after it proves itself).

## 1. Dockerfile

```dockerfile
# ---- frontend stage ----
FROM node:22 AS frontend
WORKDIR /src/web
COPY web/package.json web/yarn.lock* ./
RUN yarn install --frozen-lockfile || npm install
COPY web/ .
RUN yarn build || npm run build        # produces /src/web/dist

# ---- backend stage ----
FROM golang:1.24 AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/web/dist web/dist
RUN CGO_ENABLED=0 go build -trimpath -tags 'embed migration' \
    -o /out/paopao .

# ---- runtime stage ----
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/* \
    && useradd --system --create-home --uid 10001 websitecore
WORKDIR /app
COPY --from=backend /out/paopao /app/paopao
USER websitecore
EXPOSE 8008
ENTRYPOINT ["/app/paopao"]
CMD ["serve"]
```

If you prefer building the frontend on the host (`make build-web`) you can drop the frontend stage and simply `COPY . .` — the pre-built `web/dist/` will be embedded as long as it exists before the Go build.

## 2. Compose file

`docker-compose.prod.yml`:

```yaml
name: websitecore

services:
  app:
    build:
      context: .
      dockerfile: Dockerfile
    restart: unless-stopped
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
      meilisearch:
        condition: service_healthy
    ports:
      - "127.0.0.1:8008:8008"      # Nginx on the host proxies to this
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - appdata:/app/custom        # LocalOSS uploads + logs
    healthcheck:
      test: ["CMD-SHELL", "curl -fsS http://127.0.0.1:8008/ >/dev/null"]
      interval: 15s
      timeout: 5s
      retries: 10

  postgres:
    image: postgres:18.6
    restart: unless-stopped
    environment:
      POSTGRES_USER: websitecore
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:?set in .env}
      POSTGRES_DB: websitecore
    volumes:
      - pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U websitecore -d websitecore"]
      interval: 10s
      timeout: 5s
      retries: 10

  redis:
    image: redis:7.4.11
    restart: unless-stopped
    command: ["redis-server", "--appendonly", "yes", "--requirepass", "${REDIS_PASSWORD:?set in .env}"]
    volumes:
      - redisdata:/data
    healthcheck:
      test: ["CMD-SHELL", "redis-cli -a ${REDIS_PASSWORD} ping | grep PONG"]
      interval: 10s
      timeout: 5s
      retries: 10

  meilisearch:
    image: getmeili/meilisearch:v1.54.0
    restart: unless-stopped
    environment:
      MEILI_MASTER_KEY: ${MEILI_MASTER_KEY:?set in .env}
      MEILI_ENV: production
      MEILI_NO_ANALYTICS: "true"
    volumes:
      - meilidata:/meili_data
    healthcheck:
      test: ["CMD-SHELL", "curl -fsS http://127.0.0.1:7700/health"]
      interval: 10s
      timeout: 5s
      retries: 10

volumes:
  appdata:
  pgdata:
  redisdata:
  meilidata:
```

Secrets live in a `.env` file next to the compose file (mode `600`, never committed):

```text
POSTGRES_PASSWORD=...
REDIS_PASSWORD=...
MEILI_MASTER_KEY=...
```

## 3. Application config

Mount a `config.yaml` as shown above. Differences from the native layout in [production.md](production.md): services address each other by compose service name instead of `127.0.0.1`:

```yaml
Postgres:
  Host: postgres
Redis:
  InitAddress:
    - redis:6379
  Password: <same as REDIS_PASSWORD>
Meili:
  Host: meilisearch:7700
  ApiKey: <same as MEILI_MASTER_KEY>
LocalOSS:
  SavePath: custom/data/oss     # inside the appdata volume
```

Keep `"Migration"` in `Features.Default` (the image is built with the `migration` tag) so the schema is created on first boot. The dependency containers have no published ports at all — they are reachable only on the compose network.

## 4. Launch and verify

```sh
docker compose -f docker-compose.prod.yml up -d --wait --build
curl -fsS http://127.0.0.1:8008/
docker compose -f docker-compose.prod.yml logs -f app
```

Put Nginx + TLS in front as described in [production.md](production.md), step 6.

## Operations

```sh
# Upgrade: rebuild the app image from the new tag, dependencies keep their volumes
git checkout <release-tag>
docker compose -f docker-compose.prod.yml up -d --build app

# Database backup (same commands as database.md, container name from `docker compose ps`)
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U websitecore -Fc websitecore > backup-$(date +%F).dump

# Shell into the app container
docker compose -f docker-compose.prod.yml exec app sh
```

The `appdata` volume holds uploaded files (`custom/data/oss`) — include it in backups alongside the database dump.
