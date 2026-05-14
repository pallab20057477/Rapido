Deployment notes — backend/deployments/docker

This folder contains compose and stack configs for local development and production (Docker Swarm).

Quick local development (recommended)

1. Copy the example env file and fill values (do NOT commit secrets):

```bash
cp .env.example .env
# edit .env and set DB_PASSWORD, JWT_SECRET, REDIS_PASSWORD, GRAFANA_PASSWORD
```

2. Start services with docker-compose (builds images locally):

```bash
docker compose up --build
```

3. Stop:

```bash
docker compose down -v
```

Production with Docker Swarm (recommended professional setup)

1. Build and tag images locally (or in CI) and push to a registry accessible by your Swarm nodes. Example tags:

```bash
# from repository root
docker build -f backend/deployments/docker/Dockerfile -t myregistry/rapido-api:latest backend
# repeat for any specialized service images if needed
```

2. Create Docker secrets on manager node (do NOT store plaintext in Git):

```bash
echo "your_db_password" | docker secret create DB_PASSWORD -
echo "your_jwt_secret" | docker secret create JWT_SECRET -
echo "your_redis_password" | docker secret create REDIS_PASSWORD -
echo "your_grafana_password" | docker secret create GRAFANA_PASSWORD -
```

3. Deploy the stack:

```bash
docker stack deploy -c docker-stack.yml rapido
```

Notes & rationale

- Secrets: Use Docker secrets (or a secrets manager) so sensitive data is not exposed in environment variables or VCS.
- Entrypoint: The image contains an entrypoint that will load secrets mounted at `/run/secrets/<name>` into environment variables the app expects.
- Swarm vs Compose: `deploy:` keys (replicas, resources) are only honored by Swarm (`docker stack deploy`). `docker compose` ignores them. Use Compose for local dev and Swarm for production.
- Healthchecks: The final image includes a healthcheck; ensure your service port is responsive at `/health`.
- Ports: For production, prefer exposing only gateway/load-balancer ports and keep internal services unexposed where possible.

If you want, I can:
- Add a simple CI job to build and push images, or
- Convert the per-service builds to separate Dockerfiles/images for smaller images, or
- Replace Confluent Kafka with a lighter local-only Kafka for dev.
