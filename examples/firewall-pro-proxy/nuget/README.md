# NuGet — Firewall Pro proxy demo

Shows Sonatype Firewall Pro blocking a malicious NuGet package while an allowed version
restores normally, all through `git-pkgs proxy`.

- **Source (proxy):** `$PROXY_URL/nuget/v3/index.json`  → upstream `https://firewall.sonatype.app/nuget/`
- **Package:** `Sonatype.sonatype-policy-demo.Package`
- **Allowed:** `1.0.0`  **Blocked:** `1.1.0`, `1.2.0`, `1.3.0`

`nuget.config` in this folder points the `firewall-proxy` source at `%PROXY_URL%/nuget/v3/index.json`
(NuGet expands the env var), so set `PROXY_URL` first.

## How the block works

For NuGet, Firewall Pro lists all versions in the service index and registration, but the
malicious **`.nupkg` download returns `409 Conflict`** with a plain-text *"Sonatype … blocked
the download"* message. The proxy forwards that `409` to the client, so `dotnet`/`nuget`
restore fails on the blocked version. (npm/Maven use `403`; NuGet uses `409` — the proxy
forwards both.)

## Requirements

A .NET SDK. The sample package targets **net10.0**, so use **.NET SDK 10** for a clean
restore of the allowed version (on older SDKs the allowed `.nupkg` still downloads through
the proxy, but the build reports a target-framework mismatch — the blocking behaviour is
identical either way).

## 1. Set the proxy URL

```bash
cd examples/firewall-pro-proxy/nuget
export PROXY_URL=https://proxy.wn.leyux.de     # or your own proxy, e.g. http://localhost:8080
```

## 2. Create a scratch project and restore the allowed package (succeeds)

```bash
rm -rf demo && dotnet new classlib -n demo -o demo
cp nuget.config demo/nuget.config
cd demo
dotnet add package Sonatype.sonatype-policy-demo.Package --version 1.0.0
```

Expected: restore succeeds — `Installed Sonatype.sonatype-policy-demo.Package 1.0.0 from
$PROXY_URL/nuget/v3/index.json …`.

## 3. Restore the malicious package (blocked)

```bash
dotnet add package Sonatype.sonatype-policy-demo.Package --version 1.1.0
```

Expected: restore **fails** — the client logs `Response status code does not indicate
success: 409 (Conflict).` because Firewall Pro blocked the `.nupkg`.

Show the raw block on screen (returns the Sonatype block message):

```bash
curl -s $PROXY_URL/nuget/v3-flatcontainer/sonatype.sonatype-policy-demo.package/1.1.0/sonatype.sonatype-policy-demo.package.1.1.0.nupkg
# Sonatype has identified this component as potentially malicious and blocked the download. ...
```

## Presenter signal

On a fresh (cache-cleared) request the proxy logs on docker-wn should show the Firewall
upstream, and a block is logged explicitly:

```bash
docker logs --since 2m git-pkgs-proxy 2>&1 | grep -E 'firewall.sonatype.app/nuget|blocked by upstream policy'
```

If a fresh cache miss instead shows `https://api.nuget.org/`, the NuGet route is bypassing
Firewall Pro — pause the demo.
