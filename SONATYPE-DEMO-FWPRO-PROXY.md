# Sonatype Firewall Pro demo with git-pkgs proxy

This runbook collects demo procedures for using Sonatype Repository Firewall Pro with `git-pkgs proxy` as a third-party package registry/cache.

The docker-wn deployment exposes the proxy at:

```text
https://proxy.wn.leyux.de/
```

The proxy upstreams are configured for Sonatype Firewall Pro:

| Ecosystem | Proxy endpoint | Firewall Pro upstream | Current demo behavior |
| --- | --- | --- | --- |
| npm | `https://proxy.wn.leyux.de/npm/` | `https://firewall.sonatype.app/npm/` | malicious sample blocking works |

Before a live demo, clear or isolate the package-manager cache so local artifacts do not hide whether the proxy/Firewall path is being used.

## npm demo

Example files live in:

```text
examples/firewall-pro-proxy/npm/
```

### Clean local npm state

```bash
cd examples/firewall-pro-proxy/npm
npm cache clean --force
rm -rf node_modules package-lock.json
```

### Pull an allowed package through the proxy

```bash
npm_config_registry=https://proxy.wn.leyux.de/npm/ npm install @sonatype/policy-demo@2.0.0
```

Expected result: install succeeds. Version `2.0.0` is the normal/allowed Sonatype sample.

### Try a malicious sample through the proxy

```bash
npm_config_registry=https://proxy.wn.leyux.de/npm/ npm install @sonatype/policy-demo@2.1.0
```

Expected result: Firewall Pro blocks the request before the package is cached by the third-party registry. Versions `2.1.0`, `2.2.0`, and `2.3.0` are Sonatype sample malicious/non-normal versions; `2.0.0` is allowed.

### Presenter signal

After a fresh request, proxy logs on docker-wn should show npm upstream URLs under:

```text
https://firewall.sonatype.app/npm/
```

If logs show `https://registry.npmjs.org/` for a fresh cache miss, the npm route is not using Firewall Pro and the demo should be paused.
