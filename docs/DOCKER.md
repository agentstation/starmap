# Docker Deployment Guide

This guide covers deploying Starmap as a containerized HTTP server for production environments.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Container Architecture](#container-architecture)
- [Docker Compose](#docker-compose)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Environment Variables](#environment-variables)
- [Security Best Practices](#security-best-practices)
- [Health Checks](#health-checks)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)

## Overview

Starmap container images are:

- **Minimal**: Built with [ko](https://ko.build) on Chainguard's static base (~2MB)
- **Secure**: Zero CVEs, no shell, no package manager
- **Fast**: Direct Go compilation, no multi-stage Docker builds
- **Multi-platform**: Native support for linux/amd64 and linux/arm64

**Container Registry**: `ghcr.io/agentstation/starmap`

## Quick Start

### Run with Docker

```bash
# Basic server (embedded catalog only)
docker run -p 8080:8080 \
  ghcr.io/agentstation/starmap:latest \
  serve --host 0.0.0.0

# With API keys for live data
docker run -p 8080:8080 \
  -e OPENAI_API_KEY=sk-... \
  -e ANTHROPIC_API_KEY=sk-ant-... \
  ghcr.io/agentstation/starmap:latest \
  serve --host 0.0.0.0

# With persistent storage
docker run -p 8080:8080 \
  -v starmap-data:/home/nonroot/.starmap \
  ghcr.io/agentstation/starmap:latest \
  serve --host 0.0.0.0

# Test the server
curl http://localhost:8080/api/v1/health
```

### Version Pinning

```bash
# Latest stable
docker pull ghcr.io/agentstation/starmap:latest

# Specific version (recommended for production)
docker pull ghcr.io/agentstation/starmap:v0.0.17
docker pull ghcr.io/agentstation/starmap:0.0.17
```

## Container Architecture

### Base Image: Chainguard Static

Starmap uses `cgr.dev/chainguard/static:latest` which provides:

- **Size**: ~2MB (vs 5MB for Alpine, 124MB for Debian)
- **Security**: Zero CVEs when scanned with grype/trivy
- **Contents**: CA certificates, timezone data, /etc/passwd with nonroot user
- **No**: Shell, package manager, utilities

### Built with Ko

[Ko](https://ko.build) builds container images directly from Go source:

- No Dockerfile needed
- No Docker daemon required during build
- Native multi-platform support via Go cross-compilation
- Automatic SBOM generation (SPDX format)
- Reproducible builds with timestamp control

### Image Metadata

```bash
# Inspect image labels
docker inspect ghcr.io/agentstation/starmap:latest | jq '.[0].Config.Labels'

# View SBOM
docker pull ghcr.io/agentstation/starmap:latest
cosign verify-attestation --type spdx \
  ghcr.io/agentstation/starmap:latest
```

## Docker Compose

### Basic Setup

1. **Copy environment template:**

```bash
cp .env.example .env
```

2. **Edit .env with your configuration:**

```bash
# .env
HTTP_HOST=0.0.0.0
HTTP_PORT=8080

# Optional: Provider API keys
OPENAI_API_KEY=sk-...
ANTHROPIC_API_KEY=sk-ant-...
```

3. **Start the services:**

```bash
docker-compose up -d
```

4. **View logs:**

```bash
docker-compose logs -f starmap
```

5. **Stop services:**

```bash
docker-compose down
```

### Production Docker Compose

For production, enhance the provided `docker-compose.yml`:

```yaml
version: '3.9'

services:
  starmap:
    image: ghcr.io/agentstation/starmap:v0.0.17  # Pin version
    container_name: starmap-server

    ports:
      - "8080:8080"

    env_file:
      - .env

    volumes:
      - starmap-data:/home/nonroot/.starmap
      - ./secrets/gcp-key.json:/secrets/gcp-key.json:ro  # If using GCP

    healthcheck:
      test: ["CMD", "/ko-app/starmap", "version"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s

    restart: unless-stopped

    # Security hardening
    user: "65532:65532"
    read_only: true
    tmpfs:
      - /tmp
    cap_drop:
      - ALL
    security_opt:
      - no-new-privileges:true

    # Resource limits
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M

    # Logging
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"

volumes:
  starmap-data:
    driver: local

networks:
  default:
    driver: bridge
```

## Kubernetes Deployment

### Basic Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: starmap
  namespace: default
  labels:
    app: starmap
spec:
  replicas: 3
  selector:
    matchLabels:
      app: starmap
  template:
    metadata:
      labels:
        app: starmap
    spec:
      containers:
      - name: starmap
        image: ghcr.io/agentstation/starmap:v0.0.17
        imagePullPolicy: IfNotPresent

        command: ["/ko-app/starmap"]
        args: ["serve", "--host", "0.0.0.0", "--port", "8080"]

        ports:
        - name: http
          containerPort: 8080
          protocol: TCP

        env:
        - name: HTTP_HOST
          value: "0.0.0.0"
        - name: HTTP_PORT
          value: "8080"

        # Optional: Provider API keys from secrets
        envFrom:
        - secretRef:
            name: starmap-api-keys
            optional: true

        # Health checks
        livenessProbe:
          exec:
            command: ["/ko-app/starmap", "version"]
          initialDelaySeconds: 10
          periodSeconds: 30
          timeoutSeconds: 5
          failureThreshold: 3

        readinessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
          timeoutSeconds: 3
          failureThreshold: 2

        # Resources
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi

        # Security context
        securityContext:
          runAsUser: 65532
          runAsGroup: 65532
          runAsNonRoot: true
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL

        # Volume mounts
        volumeMounts:
        - name: tmp
          mountPath: /tmp
        - name: cache
          mountPath: /home/nonroot/.starmap

      volumes:
      - name: tmp
        emptyDir: {}
      - name: cache
        emptyDir: {}

      # Pod security
      securityContext:
        fsGroup: 65532
        seccompProfile:
          type: RuntimeDefault
```

### Service

```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: starmap
  namespace: default
  labels:
    app: starmap
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: 8080
    protocol: TCP
    name: http
  selector:
    app: starmap
```

### Ingress (Optional)

```yaml
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: starmap
  namespace: default
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - starmap.example.com
    secretName: starmap-tls
  rules:
  - host: starmap.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: starmap
            port:
              number: 8080
```

### ConfigMap for API Keys (Optional)

```yaml
# configmap.yaml
apiVersion: v1
kind: Secret
metadata:
  name: starmap-api-keys
  namespace: default
type: Opaque
stringData:
  OPENAI_API_KEY: "sk-..."
  ANTHROPIC_API_KEY: "sk-ant-..."
  GOOGLE_API_KEY: "..."
```

### Apply Kubernetes Resources

```bash
# Create namespace (optional)
kubectl create namespace starmap

# Apply resources
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
kubectl apply -f ingress.yaml

# Check deployment
kubectl get pods -n starmap
kubectl logs -f deployment/starmap -n starmap

# Test service
kubectl port-forward svc/starmap 8080:8080 -n starmap
curl http://localhost:8080/api/v1/health
```

## Environment Variables

### Server Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `HTTP_HOST` | `localhost` | Bind address (use `0.0.0.0` for containers) |
| `HTTP_PORT` | `8080` | Server port |
| `LOG_LEVEL` | `info` | Log level: trace, debug, info, warn, error |

### Provider API Keys

| Variable | Provider | Description |
|----------|----------|-------------|
| `OPENAI_API_KEY` | OpenAI | API key for OpenAI models |
| `ANTHROPIC_API_KEY` | Anthropic | API key for Claude models |
| `GOOGLE_API_KEY` | Google AI | API key for Gemini models |
| `GROQ_API_KEY` | Groq | API key for Groq models |
| `DEEPSEEK_API_KEY` | DeepSeek | API key for DeepSeek models |
| `CEREBRAS_API_KEY` | Cerebras | API key for Cerebras models |

### Google Vertex AI (Optional)

| Variable | Description |
|----------|-------------|
| `GOOGLE_VERTEX_PROJECT` | GCP project ID |
| `GOOGLE_VERTEX_LOCATION` | GCP region (e.g., `us-central1`) |
| `GOOGLE_APPLICATION_CREDENTIALS` | Path to service account key file |

### CORS Settings (Optional)

| Variable | Description |
|----------|-------------|
| `CORS_ORIGINS` | Comma-separated list of allowed origins |

## Security Best Practices

### 1. Run as Non-Root User

The Chainguard static image includes a `nonroot` user (UID 65532):

```bash
# Docker
docker run --user 65532:65532 ghcr.io/agentstation/starmap:latest

# docker-compose.yml
services:
  starmap:
    user: "65532:65532"
```

### 2. Read-Only Root Filesystem

```yaml
# docker-compose.yml
services:
  starmap:
    read_only: true
    tmpfs:
      - /tmp
```

### 3. Drop All Capabilities

```yaml
# docker-compose.yml
services:
  starmap:
    cap_drop:
      - ALL
```

### 4. Prevent Privilege Escalation

```yaml
# docker-compose.yml
services:
  starmap:
    security_opt:
      - no-new-privileges:true
```

### 5. Use Secrets for API Keys

**Docker Compose:**
```yaml
services:
  starmap:
    secrets:
      - openai_api_key
    environment:
      - OPENAI_API_KEY_FILE=/run/secrets/openai_api_key

secrets:
  openai_api_key:
    file: ./secrets/openai.key
```

**Kubernetes:**
```bash
kubectl create secret generic starmap-api-keys \
  --from-literal=OPENAI_API_KEY=sk-... \
  --from-literal=ANTHROPIC_API_KEY=sk-ant-...
```

### 6. Network Policies (Kubernetes)

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: starmap-netpol
spec:
  podSelector:
    matchLabels:
      app: starmap
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector:
        matchLabels:
          name: ingress-nginx
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 443  # HTTPS for provider APIs
```

## Health Checks

### HTTP Health Endpoint

```bash
# Check server health
curl http://localhost:8080/api/v1/health

# Expected response
{
  "status": "healthy",
  "version": "0.0.17",
  "timestamp": "2025-01-17T12:00:00Z"
}
```

### Docker Healthcheck

```yaml
healthcheck:
  test: ["CMD", "/ko-app/starmap", "version"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 10s
```

### Kubernetes Probes

```yaml
livenessProbe:
  exec:
    command: ["/ko-app/starmap", "version"]
  initialDelaySeconds: 10
  periodSeconds: 30

readinessProbe:
  httpGet:
    path: /api/v1/health
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
```

## Monitoring

### Prometheus Metrics (If Enabled)

```yaml
# ServiceMonitor for Prometheus Operator
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: starmap
spec:
  selector:
    matchLabels:
      app: starmap
  endpoints:
  - port: http
    path: /metrics
    interval: 30s
```

### Logging

**Docker Compose:**
```yaml
services:
  starmap:
    logging:
      driver: "json-file"
      options:
        max-size: "10m"
        max-file: "3"
```

**View logs:**
```bash
# Docker
docker logs -f starmap-server

# Docker Compose
docker-compose logs -f starmap

# Kubernetes
kubectl logs -f deployment/starmap
```

## Troubleshooting

### Container Won't Start

**Check logs:**
```bash
docker logs starmap-server
```

**Common issues:**
- Incorrect `HTTP_HOST` (use `0.0.0.0` for containers)
- Port already in use
- Missing environment variables
- Insufficient permissions

### Health Check Fails

```bash
# Test manually
docker exec starmap-server /ko-app/starmap version

# Check server is listening
docker exec starmap-server netstat -tlnp
```

### Connection Issues

**From host to container:**
```bash
# Verify port mapping
docker port starmap-server

# Test connection
curl -v http://localhost:8080/api/v1/health
```

**From container to provider APIs:**
```bash
# Check DNS resolution
docker exec starmap-server nslookup api.openai.com

# Check connectivity
docker exec starmap-server wget -O- https://api.openai.com
```

### Permission Denied

Ensure container runs as nonroot user:
```yaml
user: "65532:65532"
```

### Image Pull Errors

```bash
# Login to GHCR
echo $GITHUB_TOKEN | docker login ghcr.io -u USERNAME --password-stdin

# Pull with authentication
docker pull ghcr.io/agentstation/starmap:latest
```

### Debugging

**Interactive shell (not available in distroless):**

The Chainguard static image has no shell. Use debug variants for troubleshooting:

```bash
# Use a debug image with shell
docker run -it --entrypoint /bin/sh \
  cgr.dev/chainguard/busybox:latest

# Or use kubectl debug for Kubernetes
kubectl debug -it pod/starmap-xxx --image=busybox
```

---

## Additional Resources

- [Ko Documentation](https://ko.build)
- [Chainguard Images](https://edu.chainguard.dev/chainguard/chainguard-images/)
- [Starmap API Reference](REST_API.md)
- [Architecture Documentation](ARCHITECTURE.md)
- [Contributing Guide](../CONTRIBUTING.md)

## Support

- **Issues**: [GitHub Issues](https://github.com/agentstation/starmap/issues)
- **Discussions**: [GitHub Discussions](https://github.com/agentstation/starmap/discussions)
