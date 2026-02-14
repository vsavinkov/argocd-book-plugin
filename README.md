# ArgoCD Book Plugin

An ArgoCD UI extension that lets teams **book (lock) applications** for exclusive use, preventing concurrent deployment
conflicts in shared environments.

When an application is booked, a prominent indicator appears in the ArgoCD toolbar so every team member knows who is
currently working with it.

![Go](https://img.shields.io/badge/Go-1.22-blue)
![TypeScript](https://img.shields.io/badge/TypeScript-5.3-blue)
![ArgoCD](https://img.shields.io/badge/ArgoCD-2.8%2B-orange)
![License](https://img.shields.io/badge/License-Apache%202.0-green)

## How It Works

Booking state is stored as annotations directly on the ArgoCD `Application` custom resource:

```yaml
metadata:
  annotations:
    booking.argocd.io/booked-by: alice
    booking.argocd.io/booked-at: "2025-01-15T10:30:00Z"
```

No external database required. The Kubernetes API server is the single source of truth.

### UI Integration

The plugin adds two elements to the ArgoCD interface:

- **Toolbar button** — a `BOOK` / `BOOKED: username` button injected next to the Refresh button on every Application
  detail page.
- **Resource tab** — a Book tab within the Application view for a more detailed booking interface.

When booked, the button turns red and displays the booker's name. Clicking it again unbooks the application (only the
original booker or an admin can unbook).

## Features

- **Exclusive locking** — one user at a time per application
- **Admin override** — users in the `admin` group can unbook any application
- **Zero external dependencies** — state stored in Kubernetes annotations
- **Stateless backend** — scales horizontally, no database needed
- **ArgoCD-native auth** — leverages ArgoCD's proxy extension headers for user identity
- **Minimal footprint** — ~15 MB container image, 50m CPU / 32 Mi memory

## Architecture

```
┌──────────────────────────────────┐
│         ArgoCD UI                │
│  ┌────────────────────────────┐  │
│  │  extension-booking.js      │  │  Injected via init container
│  │  (BookButton + StatusPanel)│  │
│  └────────────┬───────────────┘  │
└───────────────┼──────────────────┘
                │  /extensions/booking/*
                │  (ArgoCD proxy extension)
                ▼
┌──────────────────────────────────┐
│   argocd-booking-service         │
│   Go backend (port 8080)         │
│                                  │
│   GET  /api/status               │
│   POST /api/book                 │
│   POST /api/unbook               │
│   GET  /api/list                 │
│   GET  /healthz                  │
└───────────────┬──────────────────┘
                │  Kubernetes API
                ▼
┌──────────────────────────────────┐
│  Application CRs (annotations)   │
└──────────────────────────────────┘
```

## Prerequisites

- Kubernetes cluster with **ArgoCD v2.8+** (extension proxy support)
- `kubectl` configured for your cluster
- Docker (for building the image)

## Quick Start

### 1. Build and push the image

```bash
make docker-build IMAGE_REPO=ghcr.io/yourorg/argocd-book-plugin IMAGE_TAG=0.1.0
make docker-push  IMAGE_REPO=ghcr.io/yourorg/argocd-book-plugin IMAGE_TAG=0.1.0
```

### 2. Update the image reference in the deployment manifest

Edit `manifests/deployment.yaml` and set the `image` field to match your registry:

```yaml
image: ghcr.io/yourorg/argocd-book-plugin:0.1.0
```

### 3. Deploy the backend service

```bash
make deploy
```

This applies the ServiceAccount, RBAC, Deployment, and Service manifests to the `argocd` namespace.

### 4. Patch ArgoCD to enable the extension

```bash
# Enable the extension proxy
kubectl patch cm argocd-cmd-params-cm -n argocd \
  --patch-file manifests/argocd-patches/argocd-cmd-params-cm-patch.yaml

# Register the booking backend
kubectl patch cm argocd-cm -n argocd \
  --patch-file manifests/argocd-patches/argocd-cm-patch.yaml

# Add the init container that injects the UI extension JS
kubectl patch deployment argocd-server -n argocd \
  --patch-file manifests/argocd-patches/argocd-server-patch.yaml
```

### 5. Verify

Open any Application in the ArgoCD UI. You should see a **BOOK** button in the top toolbar.

## API Reference

All endpoints are proxied through ArgoCD at `/extensions/booking/api/*`.

| Method | Path                         | Description                                  |
|--------|------------------------------|----------------------------------------------|
| `GET`  | `/api/status`                | Get booking status of an application         |
| `POST` | `/api/book`                  | Book an application for the current user     |
| `POST` | `/api/unbook`                | Unbook an application (booker or admin only) |
| `GET`  | `/api/list?namespace=argocd` | List all booked applications in a namespace  |
| `GET`  | `/healthz`                   | Health check                                 |

**Headers** (injected automatically by ArgoCD's extension proxy):

| Header                    | Example         | Description                |
|---------------------------|-----------------|----------------------------|
| `Argocd-Application-Name` | `argocd:my-app` | `namespace:appname`        |
| `Argocd-Username`         | `alice`         | Authenticated ArgoCD user  |
| `Argocd-User-Groups`      | `dev,admin`     | Comma-separated group list |
| `Argocd-Project-Name`     | `default`       | ArgoCD project             |

## Configuration

| Environment Variable | Default | Description              |
|----------------------|---------|--------------------------|
| `PORT`               | `8080`  | Backend HTTP listen port |

The admin group name is set to `admin` in the backend. Users belonging to this group can unbook applications booked by
others.

## Development

### Backend

```bash
cd backend
go test ./...     # Run tests
go vet ./...      # Lint
go build -o ../bin/server ./cmd/server   # Build binary
```

### UI

```bash
cd ui
npm ci
npm run dev       # Watch mode (development)
npm run build     # Production build
```

### Docker

```bash
make docker-build   # Builds the multi-stage image
```

## Project Structure

```
.
├── backend/
│   ├── cmd/server/main.go          # Entry point
│   └── internal/
│       ├── handler/                 # HTTP handlers + tests
│       └── k8s/                     # Kubernetes client + tests
├── ui/
│   └── src/
│       ├── index.tsx                # Extension registration
│       ├── api.ts                   # API client
│       ├── BookButton.tsx           # Book/Unbook tab component
│       └── StatusPanel.tsx          # Toolbar button component
├── manifests/                       # ArgoCD ConfigMap & Deployment patches
├── Dockerfile                       # Multi-stage build
└── Makefile
```

## Security

- Runs as **non-root** user (UID 65534)
- **Read-only root filesystem**
- All Linux capabilities **dropped**
- RBAC scoped to `get`, `list`, `patch` on `applications.argoproj.io` only
- No privilege escalation allowed
- No external network calls — communicates only with the Kubernetes API

## Uninstall

```bash
make clean
```

Then manually revert the ArgoCD patches if desired.
