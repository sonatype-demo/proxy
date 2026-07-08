# npm — Firewall Pro proxy demo

Shows Sonatype Firewall Pro blocking a malicious npm package while an allowed version
installs normally, all through `git-pkgs proxy`.

- **Registry (proxy):** `$PROXY_URL/npm/`  → upstream `https://firewall.sonatype.app/npm/`
- **Package:** `@sonatype/policy-demo`
- **Allowed:** `2.0.0`  **Blocked:** `2.1.0`, `2.2.0`, `2.3.0`

Set the proxy URL first (default is the docker-wn deployment; use your own, e.g. a local
container `http://localhost:8080`):

```bash
export PROXY_URL=https://proxy.wn.leyux.de     # or: set -a; . ../.env; set +a
```

The checked-in `package.json` has `install:allowed` / `install:blocked` scripts that target
the default docker-wn proxy; the raw commands below honor `$PROXY_URL`.

## How the block works

For npm, Firewall Pro enforces policy in two places: the malicious versions are **hidden from
the packument** (metadata), and the malicious **tarball** returns `403`. So `npm install`
either can't resolve the version or can't fetch its tarball. The proxy forwards Firewall's
`403` (with the *Sonatype Firewall Report* body) to npm instead of masking it as a `502`.

## 1. Clean local npm state

So a cached artifact can't hide the result:

```bash
cd examples/firewall-pro-proxy/npm
npm cache clean --force
rm -rf node_modules package-lock.json
```

## 2. Install an allowed package (succeeds)

```bash
npm_config_registry=$PROXY_URL/npm/ npm install @sonatype/policy-demo@2.0.0
```

Expected: install succeeds. `2.0.0` is the normal, allowed sample.

## 3. Install a malicious sample (blocked)

```bash
npm_config_registry=$PROXY_URL/npm/ npm install @sonatype/policy-demo@2.1.0
```

Expected: install **fails** — Firewall Pro blocks the component before it can be cached.
npm reports it cannot fetch the package (HTTP `403`).

Show the raw block on screen (returns the Sonatype Firewall Report JSON):

```bash
curl -s $PROXY_URL/npm/@sonatype/policy-demo/-/policy-demo-2.1.0.tgz
# {"status":403,"title":"Sonatype Firewall Report","detail":"Sonatype has identified this
#  component as potentially malicious and blocked the download. ..."}
```

## Presenter signal

On a fresh (cache-cleared) request the proxy logs on docker-wn should show the Firewall
upstream, and a block is logged explicitly:

```bash
docker logs --since 2m git-pkgs-proxy 2>&1 | grep -E 'firewall.sonatype.app/npm|blocked by upstream policy'
```

If a fresh cache miss instead shows `https://registry.npmjs.org/`, the npm route is bypassing
Firewall Pro — pause the demo.
