# Ollama DDOS Culler

Automated script to cull unauthorized Ollama models by stopping and deleting them every X seconds. Designed for high-frequency polling (5 seconds) to quickly remove abusive models.

You shouldn't ever need this. Protect your Ollama instance!

## Features

- **Fast polling**: Runs every 5 seconds (configurable)
- **API-based**: No CLI dependency, uses Ollama REST API
- **Distroless Docker**: ~15MB image, minimal attack surface
- **Whitelist support**: Protect specific models
- **Work hours**: Optional time-based culling
- **Continuous deployment**: GitHub Actions → GHCR

## Quick Start

### Local Development

```bash
# Create .env file (copy from .env.example)
cp .env.example .env
# Edit .env with your OLLAMA_HOST

# Run with docker-compose
docker-compose up -d
```

### Docker

```bash
# Build
docker build -t ollama-culler .

# Run
docker run -d \
  --name ollama-culler \
  -e OLLAMA_HOST=http://localhost:11434 \
  -e WHITELIST_MODELS=gemma3:12b \
  -e POLL_INTERVAL=5 \
  ollama-culler
```

### Kubernetes (Bjw-s Helm Chart)

```bash
helm repo add bjw-s https://bjw-s.github.io/helm-charts/
helm install ollama-culler bjw-s/common \
  -f docs/helm-values.yaml
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `OLLAMA_HOST` | Yes | `http://localhost:11434` | Ollama API endpoint |
| `WHITELIST_MODELS` | No | `gemma3:12b` | Comma-separated models to protect |
| `POLL_INTERVAL` | No | `5` | Seconds between checks (1-3600) |
| `ENABLE_WORK_HOURS` | No | `false` | Enable time-based culling |
| `WORK_HOURS_START` | No | `09:00` | Work window start (HH:MM) |
| `WORK_HOURS_END` | No | `17:00` | Work window end (HH:MM) |
| `TZ` | No | Server local | IANA timezone |

### Work Hours Configuration

Work hours are handled automatically based on your server's timezone:

```bash
# Protect models during business hours (9 AM - 5 PM local time)
ENABLE_WORK_HOURS=true
WORK_HOURS_START=09:00
WORK_HOURS_END=17:00

# Overnight protection (10 PM - 6 AM)
WORK_HOURS_START=22:00
WORK_HOURS_END=06:00

# Change timezone
TZ=America/New_York
```

## Whitelist Examples

```bash
# Single model
WHITELIST_MODELS=gemma3:12b

# Multiple models
WHITELIST_MODELS=gemma3:12b,llama3.1:7b,mistral:7b

# Model prefixes (must match exactly)
WHITELIST_MODELS=llama3*,custom*
```

## Docker Image

Build and push to GitHub Container Registry:

```bash
# Push to GHCR
docker build -t ghcr.io/USERNAME/ollama-ddos:latest .
docker push ghcr.io/USERNAME/ollama-ddos:latest
```

CI/CD automatically builds and pushes to `ghcr.io/USERNAME/ollama-ddos` on:
- Push to `main` branch (tags as `latest`)
- Creation of semantic version tags (e.g., `v1.0.0`)

## Deployment to Kubernetes

### Using Bjw-s Chart

```yaml
# docs/helm-values.yaml
controllers:
  culler:
    type: deployment
    containers:
      culler:
        image:
          repository: ghcr.io/USERNAME/ollama-ddos
          tag: latest
        env:
          - name: OLLAMA_HOST
            value: "http://localhost:11434"
          - name: WHITELIST_MODELS
            value: "gemma3:12b,llama3.1:7b"
          - name: POLL_INTERVAL
            value: "5"
          - name: TZ
            value: "America/New_York"
```

### Standalone Deployment

```bash
kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ollama-culler
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ollama-culler
  template:
    metadata:
      labels:
        app: ollama-culler
    spec:
      serviceAccountName: ollama-culler
      containers:
        - name: culler
          image: ghcr.io/USERNAME/ollama-ddos:latest
          env:
            - name: OLLAMA_HOST
              value: "http://localhost:11434"
            - name: WHITELIST_MODELS
              value: "gemma3:12b"
            - name: POLL_INTERVAL
              value: "5"
            - name: TZ
              value: "America/New_York"
          resources:
            requests:
              cpu: 10m
              memory: 32Mi
            limits:
              cpu: 50m
              memory: 64Mi
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ollama-culler
EOF
```

## Logging

Logs are written to stdout in structured format:

```json
{"time":"2026-03-06T21:30:00Z","level":"INFO","msg":"Starting Ollama culler","host":"http://localhost:11434","whitelist":{"gemma3:12b":true}}
{"time":"2026-03-06T21:30:00Z","level":"INFO","msg":"Stopping and removing model","model":"gemma2:9b","timestamp":"2026-03-06 21:30:00"}
{"time":"2026-03-06T21:30:00Z","level":"DEBUG","msg":"Skipping whitelisted model","model":"gemma3:12b","timestamp":"2026-03-06 21:30:00"}
```

## Troubleshooting

### Ollama API not reachable

Check connectivity:
```bash
curl http://localhost:11434/api/ps
```

Ensure OLLAMA_HOST is correct and accessible from the deployment environment.

### Models not being deleted

Check logs for errors:
```bash
kubectl logs -l app=ollama-culler
```

Verify the Ollama API is responding and the model names match exactly (including version tag).

### Work hours not working

Ensure `TZ` is set correctly:
```bash
kubectl exec -it ollama-culler-xxxx -- date
kubectl exec -it ollama-culler-xxxx -- cat /etc/timezone
```

## Security

- Environment variables are used for secrets (never hardcoded)
- Distroless image minimizes attack surface
- Non-root user in container
- Read-only filesystem (no write access needed)

## License

WTFPL
