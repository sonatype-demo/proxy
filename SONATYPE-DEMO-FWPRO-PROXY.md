# Sonatype Firewall Pro demo with git-pkgs proxy

## What this is

[`git-pkgs proxy`](README.md) is a caching proxy for package registries (npm, PyPI,
Maven, and many more). Upstream, it points each ecosystem at its public registry
(registry.npmjs.org, PyPI, Maven Central, …).

**This fork repoints the npm, PyPI and Maven upstreams at [Sonatype Repository
Firewall](https://www.sonatype.com/products/sonatype-repository-firewall) ("Firewall
Pro") instead.** Firewall Pro evaluates every requested component against Sonatype
policy and **quarantines malicious or non-compliant versions**. Placing the proxy in
front of it gives developers a single, ordinary-looking registry URL per ecosystem
while every download is transparently screened by Firewall Pro.

The demo shows, live, that:

- **known-good** package versions install normally through the proxy, and
- **known-malicious** sample versions are **blocked** — for all three ecosystems.

```
  npm / pip / mvn                git-pkgs proxy                 Sonatype Firewall Pro
  ────────────────>       $PROXY_URL/*         ──────>  https://firewall.sonatype.app/*
   (plain registry URL,          (caches allowed                (evaluates policy,
    no auth needed)               artifacts, forwards            quarantines malicious
                                  policy blocks)                 versions, holds creds)
```

## The docker-wn deployment

The proxy runs as a Docker Compose service on `docker-wn` and is published at
`https://proxy.wn.leyux.de/`. Deployment repo: `/home/cleygraf/git/wn-leyux-org/proxy`
(see its README). The Firewall Pro basic-auth credentials are injected into the
container from that repo's untracked `.env`. A gitignored copy of that `.env` — plus the
non-secret `PROXY_URL` below — also lives at `examples/firewall-pro-proxy/.env` so the demo
and verification script can be run self-contained (see below). Never commit or print the
credential values.

### Point the demo at any proxy — `PROXY_URL`

The demo `.env` defines a non-secret **`PROXY_URL`**: the base URL of the proxy the demo
commands talk to (no trailing slash). It defaults to the docker-wn deployment:

```bash
PROXY_URL=https://proxy.wn.leyux.de
```

Set it to your own proxy to run the same demo elsewhere — for example a local proxy
container: `PROXY_URL=http://localhost:8080`. Every command in this runbook and the
per-ecosystem READMEs uses `$PROXY_URL`. Load it with `set -a; . ./.env; set +a` from
`examples/firewall-pro-proxy/` (this also loads the Firewall creds for the verify script),
or, for the through-proxy demos that need no credentials, just `export PROXY_URL=…`.

| Ecosystem | Proxy endpoint (what developers use) | Firewall Pro upstream                  |
| --------- | ------------------------------------ | -------------------------------------- |
| npm       | `$PROXY_URL/npm/`                     | `https://firewall.sonatype.app/npm/`   |
| PyPI      | `$PROXY_URL/pypi/simple/`             | `https://firewall.sonatype.app/pypi/`  |
| Maven     | `$PROXY_URL/maven/`                   | `https://firewall.sonatype.app/mvn/`   |

## The sample packages

Sonatype publishes `policy-demo` packages whose versions are deliberately flagged so a
Firewall policy can block them. One version per ecosystem is normal; the rest are
non-normal/malicious.

| Ecosystem | Package                          | Allowed | Blocked (non-normal)      |
| --------- | -------------------------------- | ------- | ------------------------- |
| npm       | `@sonatype/policy-demo`          | `2.0.0` | `2.1.0`, `2.2.0`, `2.3.0` |
| PyPI      | `python-policy-demo`             | `1.0.0` | `1.1.0`, `1.2.0`, `1.3.0` |
| Maven     | `org.sonatype:maven-policy-demo` | `1.0.0` | `1.1.0`, `1.2.0`, `1.3.0` |

## How a block appears at the HTTP level

Firewall Pro does not enforce policy the same way in every ecosystem, which matters for
demoing it correctly:

| Ecosystem | Where the block happens                              | What the client sees                        |
| --------- | ---------------------------------------------------- | ------------------------------------------- |
| npm       | The malicious **tarball** returns `403`; the version is also hidden from the packument | `npm install` fails to fetch the tarball    |
| PyPI      | The malicious version is **omitted from the PEP 503 simple index** (no download link) | pip: `No matching distribution found`       |
| Maven     | The **JAR** (component binary) returns `403`; the **POM is always served (200)** | `mvn` fails to transfer the artifact        |

The Maven distinction is the easy one to get wrong: the POM is only metadata and is never
quarantined, so a `:pom` request can never demonstrate a block. **Always demo Maven with
the `:jar`.**

## What this fork changes, and why

The proxy needed both configuration and source changes to front Firewall Pro correctly.

### 1. Repoint the upstreams at Firewall Pro (enablement)

The stock proxy hard-codes public registries. This fork lets the npm and PyPI upstreams be
configured and rewrites Firewall's artifact links back through the proxy
(`ea25a6b`, `73e035c`), and the deployment config sets `upstream.npm/pypi/maven` to the
Firewall Pro endpoints with basic-auth. Result: every fresh request is screened by
Firewall Pro rather than the public registry.

### 2. Demo the Maven block on the JAR, not the POM

The original Maven demo fetched `:pom`, which Firewall always serves (`200`), so "blocking
was not observed." The runbook and sample `pom.xml` now use the **JAR**, which is the
artifact Firewall actually quarantines. (Root-caused with direct-to-Firewall probes; see
the Maven README.)

### 3. Forward Firewall's `403` to the client, consistently (`27498bd`, `762ff95`)

When Firewall quarantines a component it returns `403` with a JSON *Sonatype Firewall
Report* body. The stock proxy mapped that to a generic **`502 Bad Gateway`**, hiding the
reason. Now the proxy **forwards the `403` and the report body** to the client. A shared
helper (`serveUpstreamBlock` / `writeArtifactError` in `internal/handler/handler.go`) is
adopted by **every** ecosystem download handler — so blocking behaves the same whether you
pull npm, Maven, or any other ecosystem — with npm keeping its JSON error shape and the OCI
handler emitting a native `403 DENIED`. Maven now reports `status code: 403, reason phrase:
Forbidden (403)` instead of `502`.

### 4. Don't let policy blocks trip the circuit breaker (`9f396da`)

The proxy's upstream fetcher has a per-host circuit breaker. The stock breaker counts
**every** non-2xx as a failure — including a `403` policy block — and because one host
(`firewall.sonatype.app`) fronts all three ecosystems, they share one breaker. Blocking a
handful of malicious packages in a row would open it after 5 failures and then **healthy,
allowed packages would start failing with `502`** for the backoff window.

`internal/fetchcb` replaces that with a **policy-aware breaker**: a `4xx` client response (a
`404`, or any `403` block) is returned to the caller but **does not count as a failure**;
only genuine unavailability (`5xx`, rate limiting, transport errors) trips the breaker. A
run of dozens of consecutive blocks no longer disrupts allowed traffic. Same tuning as the
stock breaker (per-host, threshold 5, 30 s→5 min backoff).

### 5. A one-shot verification script (`0f29f29`)

`examples/firewall-pro-proxy/verify-firewall-blocking.sh` asserts the whole matrix in one
run. See below.

## Running the demo

Per-ecosystem, hands-on runbooks live next to the example projects:

- **npm** — [`examples/firewall-pro-proxy/npm/README.md`](examples/firewall-pro-proxy/npm/README.md)
- **PyPI** — [`examples/firewall-pro-proxy/pypi/README.md`](examples/firewall-pro-proxy/pypi/README.md)
- **Maven** — [`examples/firewall-pro-proxy/maven/README.md`](examples/firewall-pro-proxy/maven/README.md)

Overview and shared setup: [`examples/firewall-pro-proxy/README.md`](examples/firewall-pro-proxy/README.md).

Before a live demo, clear or isolate the package-manager cache (each README shows how) so a
locally cached artifact can't hide whether the proxy/Firewall path was actually exercised.

## Automated verification

`examples/firewall-pro-proxy/verify-firewall-blocking.sh` checks, in one run, that the
malicious `policy-demo` versions are blocked and the allowed ones are served, for all three
ecosystems. It downloads no package bytes and is safe to run in CI.

By default it talks **directly to Firewall Pro** (isolating Firewall/policy from the proxy):

```bash
cd examples/firewall-pro-proxy
set -a; . ./.env; set +a   # loads Firewall creds + PROXY_URL (gitignored local copy)
./verify-firewall-blocking.sh
```

Point it **through the proxy** to verify the end-to-end path (this is what exercises the
`403`-forwarding and breaker changes above). The upstreams are derived from `$PROXY_URL`, so
this works against any proxy — the docker-wn default or your own `http://localhost:8080`:

```bash
set -a; . ./.env; set +a   # loads Firewall creds + PROXY_URL
FIREWALL_BASE=$PROXY_URL \
NPM_UPSTREAM=$PROXY_URL/npm \
PYPI_UPSTREAM=$PROXY_URL/pypi \
MAVEN_UPSTREAM=$PROXY_URL/maven \
./verify-firewall-blocking.sh
```

It reads `SONATYPE_FIREWALL_USERNAME` / `SONATYPE_FIREWALL_PASSWORD` from the environment
(never printed), exits `0` when every expectation holds and `1` if any malicious version is
served or any allowed version is blocked.

## Presenter signal — confirm Firewall Pro is really in the path

A mounted config is not proof that Firewall is being used; a cached artifact can serve
without any upstream call. After a **fresh** (cache-cleared) request, the proxy logs on
docker-wn should show the Firewall upstream:

```bash
docker logs --since 2m git-pkgs-proxy 2>&1 \
  | grep -E 'firewall.sonatype.app|registry.npmjs.org|files.pythonhosted.org|repo1.maven.org'
```

| Ecosystem | Good (Firewall in path)                       | Bad (bypassing Firewall)              |
| --------- | --------------------------------------------- | ------------------------------------- |
| npm       | `https://firewall.sonatype.app/npm/`          | `https://registry.npmjs.org/`         |
| PyPI      | `https://firewall.sonatype.app/pypi/packages/`| `https://files.pythonhosted.org/`     |
| Maven     | `https://firewall.sonatype.app/mvn/`          | `https://repo1.maven.org/maven2/`     |

A confirmed block is logged as `artifact blocked by upstream policy` with the Firewall
report `detail`. If you see a `registry.npmjs.org` / `files.pythonhosted.org` /
`repo1.maven.org` URL on a fresh cache miss, the route is bypassing Firewall — pause the
demo.

## Gradle note (not part of the blocking demo)

`examples/firewall-pro-proxy/maven/settings.gradle.kts` shows Gradle plugin resolution
through the Maven endpoint, and the separate `/gradle/` **HTTP Build Cache** endpoint. The
build cache is unrelated to package blocking. Gradle Plugin Portal fallback is intentionally
disabled for this demo (`a2d8314`), so Gradle plugin markers only resolve if Firewall serves
them — do not present Gradle plugin resolution as part of the Firewall Pro blocking demo. See
the Maven README for detail.
