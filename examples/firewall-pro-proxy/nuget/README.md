# NuGet — Firewall Pro proxy demo

Shows Sonatype Firewall Pro blocking a malicious NuGet package while an allowed version
restores normally, all through `git-pkgs proxy`.

> [!IMPORTANT]
> **.NET SDK 10 is required for this demo.** The sample package supports only `net10.0`;
> .NET SDK 9 creates a `net9.0` project and the allowed package restore ends with `NU1202`.

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

## Requirement: .NET SDK 10

This requirement is mandatory for the real `dotnet restore` scenario. Confirm that SDK 10
is installed and active before creating the project:

```bash
dotnet --version
dotnet --list-sdks
```

The active version must start with `10.`. With SDK 9, the allowed `.nupkg` still downloads
successfully through the proxy, but NuGet then fails the project compatibility check with
`NU1202` because a `net9.0` project cannot consume a package that supports only `net10.0`.
That is a local SDK/framework failure, not a proxy or Firewall failure. If SDK 10 is not
available, use the top-level `verify-firewall-blocking.sh` HTTP-status verification instead
of presenting the real restore scenario.

## 1. Set the proxy URL

```bash
cd examples/firewall-pro-proxy/nuget
export PROXY_URL=https://proxy.wn.leyux.de     # or your own proxy, e.g. http://localhost:8080
```

The included local Compose proxy serves **plain HTTP**, so use
`PROXY_URL=http://localhost:8080`, not `https://localhost:8080`. Using HTTPS against that
HTTP listener produces TLS errors such as `Cannot determine the frame size or a corrupted
frame was received`. The checked-in `nuget.config` sets `allowInsecureConnections="true"`
for this source because current NuGet clients otherwise reject HTTP registries. HTTPS proxy
URLs continue to use normal certificate validation.

## 2. Create a scratch project and restore the allowed package (succeeds)

Run these commands from `examples/firewall-pro-proxy/nuget`, not from an existing `demo`
directory. The resulting project path should end in `nuget/demo/demo.csproj`; a path such as
`nuget/demo/demo/demo.csproj` means the setup was started one directory too deep.

```bash
cd /home/cleygraf/git/proxy/examples/firewall-pro-proxy/nuget
rm -rf demo
dotnet new classlib --framework net10.0 --name demo --output demo
cp nuget.config demo/nuget.config
cd demo
dotnet add package Sonatype.sonatype-policy-demo.Package --version 1.0.0
```

Expected: restore succeeds — `Installed Sonatype.sonatype-policy-demo.Package 1.0.0 from
`$PROXY_URL/nuget/v3/index.json …`.

How to read the output:

- `GET ...1.0.0.nupkg`, `OK`, and `Installed ... 1.0.0` prove that transport through the
  proxy worked.
- A later `NU1202` means the project used the wrong target framework/SDK; use SDK 10 and
  recreate the project with `--framework net10.0`.
- `NU1900` for `$PROXY_URL/nuget/v3/vulnerabilities/index.json` is currently a non-fatal
  audit warning: the demo proxy does not expose that optional NuGet vulnerability resource.
  It is unrelated to package routing and Firewall policy blocking.

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
