# Firewall Pro proxy demo — examples

Ready-to-run example projects for demonstrating **Sonatype Repository Firewall Pro**
through `git-pkgs proxy`. Each subfolder is a minimal project for one ecosystem that
installs a known-good package (succeeds) and a known-malicious sample (blocked), pulling
through the proxy at `https://proxy.wn.leyux.de/`.

Start with the top-level runbook: [`../../SONATYPE-DEMO-FWPRO-PROXY.md`](../../SONATYPE-DEMO-FWPRO-PROXY.md)
— it explains the purpose, architecture, and what this fork changed.

## Layout

| Path                              | What it is                                                        |
| --------------------------------- | ---------------------------------------------------------------- |
| [`npm/`](npm/README.md)           | npm demo — `@sonatype/policy-demo`                               |
| [`pypi/`](pypi/README.md)         | PyPI/pip demo — `python-policy-demo`                             |
| [`maven/`](maven/README.md)       | Maven demo — `org.sonatype:maven-policy-demo` (+ Gradle notes)  |
| `verify-firewall-blocking.sh`     | One-shot check of the whole allowed/blocked matrix (all 3)       |

## The demo in one line per ecosystem

| Ecosystem | Allowed (installs)                       | Blocked (fails)                          |
| --------- | ---------------------------------------- | ---------------------------------------- |
| npm       | `@sonatype/policy-demo@2.0.0`            | `@sonatype/policy-demo@2.1.0`            |
| PyPI      | `python-policy-demo==1.0.0`              | `python-policy-demo==1.1.0`             |
| Maven     | `org.sonatype:maven-policy-demo:1.0.0:jar` | `org.sonatype:maven-policy-demo:1.1.0:jar` |

## Automated verification (all three at once)

`verify-firewall-blocking.sh` asserts that every malicious sample is blocked and every
allowed sample is served. It downloads no package bytes and is safe to run in CI.

```bash
# Credentials are only needed to talk DIRECTLY to Firewall Pro (the default target).
set -a; . /home/cleygraf/git/wn-leyux-org/proxy/.env; set +a   # load Firewall creds
./verify-firewall-blocking.sh                                  # direct to Firewall Pro
```

Run it **through the proxy** to verify the end-to-end path developers actually use:

```bash
FIREWALL_BASE=https://proxy.wn.leyux.de \
NPM_UPSTREAM=https://proxy.wn.leyux.de/npm \
PYPI_UPSTREAM=https://proxy.wn.leyux.de/pypi \
MAVEN_UPSTREAM=https://proxy.wn.leyux.de/maven \
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
`https://proxy.wn.leyux.de/*` URLs. Firewall Pro basic-auth credentials are held only by
the proxy (in the deployment repo's untracked `.env`) and are needed **only** to run the
verification script in its default "direct to Firewall" mode. Never commit or print them.
