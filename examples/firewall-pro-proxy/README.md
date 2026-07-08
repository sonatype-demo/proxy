# Firewall Pro proxy demo — examples

Ready-to-run example projects for demonstrating **Sonatype Repository Firewall Pro**
through `git-pkgs proxy`. Each subfolder is a minimal project for one ecosystem that
installs a known-good package (succeeds) and a known-malicious sample (blocked), pulling
through the proxy at **`$PROXY_URL`** (see Configuration below).

Start with the top-level runbook: [`../../SONATYPE-DEMO-FWPRO-PROXY.md`](../../SONATYPE-DEMO-FWPRO-PROXY.md)
— it explains the purpose, architecture, and what this fork changed.

## Configuration — `PROXY_URL`

Every command here targets the proxy through **`$PROXY_URL`**, the proxy base URL (no
trailing slash). It is defined in this folder's `.env` and defaults to the docker-wn
deployment; change that one line to point at your own proxy — e.g. a local container:

```bash
PROXY_URL=https://proxy.wn.leyux.de      # default (docker-wn)
# PROXY_URL=http://localhost:8080        # e.g. a local proxy container
```

Load it before running the commands:

```bash
set -a; . ./.env; set +a     # loads PROXY_URL (+ Firewall creds, needed only by the verify script)
# or, for the through-proxy demos that need no credentials:
export PROXY_URL=https://proxy.wn.leyux.de
```

Want to run against your **own** proxy instead of the docker-wn deployment? Spin up a local
proxy container (Rancher Desktop) — see [`local-proxy/`](local-proxy/README.md) — and set
`PROXY_URL=http://localhost:8080`.

`.env` holds three values: `PROXY_URL` (non-secret) and the two Firewall Pro basic-auth
credentials `SONATYPE_FIREWALL_USERNAME` / `SONATYPE_FIREWALL_PASSWORD`. It is gitignored
and never committed. A committed **`.env.example`** template ships with sample values — copy
it and fill in your own:

```bash
cp .env.example .env      # then edit PROXY_URL and the Firewall creds
```

## Layout

| Path                              | What it is                                                        |
| --------------------------------- | ---------------------------------------------------------------- |
| [`npm/`](npm/README.md)           | npm demo — `@sonatype/policy-demo`                               |
| [`pypi/`](pypi/README.md)         | PyPI/pip demo — `python-policy-demo`                             |
| [`maven/`](maven/README.md)       | Maven demo — `org.sonatype:maven-policy-demo` (+ Gradle notes)  |
| [`nuget/`](nuget/README.md)       | NuGet demo — `Sonatype.sonatype-policy-demo.Package`            |
| [`local-proxy/`](local-proxy/README.md) | Run your **own** proxy container locally (Rancher Desktop) at `localhost:8080` |
| `verify-firewall-blocking.sh`     | One-shot check of the whole allowed/blocked matrix (all 4)       |

## The demo in one line per ecosystem

| Ecosystem | Allowed (installs)                       | Blocked (fails)                          |
| --------- | ---------------------------------------- | ---------------------------------------- |
| npm       | `@sonatype/policy-demo@2.0.0`            | `@sonatype/policy-demo@2.1.0`            |
| PyPI      | `python-policy-demo==1.0.0`              | `python-policy-demo==1.1.0`             |
| Maven     | `org.sonatype:maven-policy-demo:1.0.0:jar` | `org.sonatype:maven-policy-demo:1.1.0:jar` |
| NuGet     | `Sonatype.sonatype-policy-demo.Package` 1.0.0 | `Sonatype.sonatype-policy-demo.Package` 1.1.0 |

## Automated verification (all four at once)

`verify-firewall-blocking.sh` asserts that every malicious sample is blocked and every
allowed sample is served. It downloads no package bytes and is safe to run in CI.

```bash
# Credentials are only needed to talk DIRECTLY to Firewall Pro (the default target).
set -a; . ./.env; set +a   # loads Firewall creds + PROXY_URL (gitignored local copy)
./verify-firewall-blocking.sh                                  # direct to Firewall Pro
```

Run it **through the proxy** to verify the end-to-end path developers actually use (the
upstreams are derived from `$PROXY_URL`, so it works against any proxy):

```bash
set -a; . ./.env; set +a   # loads Firewall creds + PROXY_URL
FIREWALL_BASE=$PROXY_URL \
NPM_UPSTREAM=$PROXY_URL/npm \
PYPI_UPSTREAM=$PROXY_URL/pypi \
MAVEN_UPSTREAM=$PROXY_URL/maven \
NUGET_UPSTREAM=$PROXY_URL/nuget \
./verify-firewall-blocking.sh
```

Exit `0` = every expectation held; `1` = a malicious version was served or an allowed
version was blocked; `2` = setup error (missing creds / `curl`). Credentials are read from
`SONATYPE_FIREWALL_USERNAME` / `SONATYPE_FIREWALL_PASSWORD` and are never printed.

## Before a live demo

Clear or isolate the package-manager cache first, so a locally cached artifact can't hide
whether the proxy/Firewall path was actually used. Each ecosystem README shows the exact
commands. Confirm Firewall is in the path by checking the proxy logs for
`firewall.sonatype.app` upstream URLs on a fresh request (see the top-level runbook,
"Presenter signal").

## Credentials

The demo endpoints on the proxy need **no** authentication — developers just use the
`$PROXY_URL/*` URLs. Firewall Pro basic-auth credentials are needed **only** to run the
verification script in its default "direct to Firewall" mode. The `.env` in this folder
(`examples/firewall-pro-proxy/.env`) holds `PROXY_URL` plus a copy of the Firewall Pro
credentials; it is gitignored (never committed) and holds the same live basic-auth token as
the deployment repo. Source it with `. ./.env` before running the script. Never commit or
print the credential values.
