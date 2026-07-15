# Local proxy container (Rancher Desktop)

Run your **own** git-pkgs proxy — fronting Sonatype Firewall Pro — as a local container at
`http://localhost:8080`, then point the demo at it with `PROXY_URL=http://localhost:8080`.
This lets you run the whole blocking demo without the shared docker-wn deployment.

Files here:

| File                 | Purpose                                                             |
| -------------------- | ------------------------------------------------------------------ |
| `docker-compose.yml` | Builds the proxy image from source and serves it on port 8080      |
| `config.yml`         | Proxy config: npm/PyPI/Maven/NuGet upstreams → Firewall Pro, SQLite + local storage |

The proxy screens packages through Firewall Pro, so you need **Firewall Pro basic-auth
credentials** (same as the verify script). Without them, upstream auth fails and packages
won't resolve.

## Prerequisites — Rancher Desktop

[Rancher Desktop](https://rancherdesktop.io/) provides the container runtime. Pick a
container engine in **Preferences → Container Engine**:

- **dockerd (moby)** — use the `docker` / `docker compose` commands below (recommended).
- **containerd** — use `nerdctl compose` in place of `docker compose`.

Kubernetes is not needed; you can disable it (Preferences → Kubernetes) to save resources.
Rancher Desktop automatically forwards published container ports to `localhost`, so
`8080` is reachable as `http://localhost:8080` with no extra setup.

## 1. Provide credentials

From the demo folder (`examples/firewall-pro-proxy/`), create the gitignored `.env` from the
template and fill in your values:

```bash
cd ..                       # examples/firewall-pro-proxy/
cp .env.example .env        # then edit:
#   PROXY_URL=http://localhost:8080
#   SONATYPE_FIREWALL_USERNAME=<your Firewall Pro user>
#   SONATYPE_FIREWALL_PASSWORD=<your Firewall Pro token>
cd local-proxy
```

`docker-compose.yml` reads `../.env` for the Firewall credentials (they are injected into the
container; `config.yml` references them as `${SONATYPE_FIREWALL_USERNAME}` /
`${SONATYPE_FIREWALL_PASSWORD}`).

## 2. Build and start the proxy

The image is built locally from the repo's `Dockerfile` (a multi-stage Go build → minimal
Alpine runtime). `--build` builds it on first run:

```bash
docker compose up -d --build      # moby backend
# nerdctl compose up -d --build   # containerd backend
```

## 3. Verify it's up

```bash
curl http://localhost:8080/health          # -> 200
# web UI: http://localhost:8080/ui/
docker compose logs -f proxy               # startup / request logs
```

## 4. Run the demo against your local proxy

`PROXY_URL` already points at it (from `.env`), so just follow the ecosystem runbooks or the
one-shot verifier:

The local listener is plain HTTP. Keep `PROXY_URL=http://localhost:8080`; do not change it
to `https://localhost:8080`. The NuGet example explicitly permits this local HTTP source via
`allowInsecureConnections="true"` in its checked-in `nuget.config`.

```bash
cd ..                                       # examples/firewall-pro-proxy/
set -a
. ./.env
. ./.env_proxy
set +a
./verify-firewall-blocking.sh               # expect 16 passed, 0 failed
```

`localhost` means the machine or container where the package-manager command runs. If the
client runs inside another container, `localhost:8080` points back to that client container,
not to the Docker host. For example, inside docker-wn's `code-server` container, port 8080 is
code-server itself. Source `.env` first, then override its host-side URL before deriving the
ecosystem endpoints:

```bash
cd ..                                       # examples/firewall-pro-proxy/
set -a; . ./.env; set +a
HOST_GATEWAY=$(ip route | awk '/default/ {print $3; exit}')
export PROXY_URL="http://$HOST_GATEWAY:8080"
set -a; . ./.env_proxy; set +a
curl "$PROXY_URL/health"
./verify-firewall-blocking.sh
```

On the docker-wn network used during verification this gateway was `172.20.0.1`, but derive
it instead of hard-coding it because Docker networks can change.

or a quick manual check:

```bash
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/npm/@sonatype/policy-demo/-/policy-demo-2.0.0.tgz   # 200 allowed
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/npm/@sonatype/policy-demo/-/policy-demo-2.1.0.tgz   # 403 blocked
```

## 5. Stop / clean up

```bash
docker compose down          # stop and remove the container (keeps cached artifacts)
docker compose down -v       # also drop the proxy-data volume (SQLite DB + cache)
```

## Building the custom image by hand

`docker compose --build` is the easy path, but you can build and iterate on the image
directly. From the **repo root** (the build context is the whole repo, since the Go build
needs the sources):

```bash
cd ../../..                                 # repo root (has the Dockerfile)
docker build -t git-pkgs-proxy:local .      # moby
# nerdctl build -t git-pkgs-proxy:local .   # containerd
```

`docker-compose.yml` uses `image: git-pkgs-proxy:local`, so a subsequent
`docker compose up -d` (without `--build`) reuses whatever you built. After changing source,
rebuild with `docker compose build --no-cache` (or `docker compose up -d --build`).

What the Dockerfile does:

1. **builder stage** (`golang:*-alpine`) — `go mod download`, then
   `CGO_ENABLED=0 go build` produces a single static `proxy` binary.
2. **runtime stage** (`alpine`) — copies just the binary, adds CA certificates, runs as a
   non-root `proxy` user, exposes `8080`, and `serve`s.

## Notes for Rancher Desktop

- **Which CLI:** `docker compose` needs the **dockerd (moby)** backend. On **containerd**,
  substitute `nerdctl compose` for every `docker compose` command above; `docker build`
  becomes `nerdctl build`.
- **Apple Silicon / ARM:** the build runs natively (arm64) inside Rancher Desktop's Lima VM —
  no `platform` override needed for local use.
- **Data location:** the `proxy-data` volume lives inside the Rancher Desktop VM; `down -v`
  removes it.
- **Port already in use:** if something else holds `8080`, change the left side of the port
  mapping (e.g. `"18080:8080"`) and set `PROXY_URL=http://localhost:18080` to match.
