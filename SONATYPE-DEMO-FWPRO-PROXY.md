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
| PyPI | `https://proxy.wn.leyux.de/pypi/simple/` | `https://firewall.sonatype.app/pypi/` | malicious sample blocking works |
| Maven | `https://proxy.wn.leyux.de/maven/` | `https://firewall.sonatype.app/mvn/` | malicious sample blocking works — enforced on the **JAR**, not the POM |

Before a live demo, clear or isolate the package-manager cache so local artifacts do not hide whether the proxy/Firewall path is being used.

## Automated verification (direct Firewall Pro)

`examples/firewall-pro-proxy/verify-firewall-blocking.sh` checks in one run that
Firewall Pro blocks the malicious `policy-demo` sample versions and serves the
allowed ones for all three ecosystems. It talks directly to Firewall Pro (not
through the proxy), downloads no package bytes, and is safe to run in CI.

```bash
cd examples/firewall-pro-proxy
set -a; . /home/cleygraf/git/wn-leyux-org/proxy/.env; set +a   # load Firewall creds
./verify-firewall-blocking.sh
```

It reads `SONATYPE_FIREWALL_USERNAME` / `SONATYPE_FIREWALL_PASSWORD` from the
environment (never printed), exits `0` when every expectation holds and `1` if
any malicious version is served or any allowed version is blocked. Blocking
signals checked: npm tarball and Maven JAR return `403`; the PyPI malicious
versions are absent from the PEP 503 simple index.

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


## PyPI / pip demo

Example files live in:

```text
examples/firewall-pro-proxy/pypi/
```

On docker-wn, the host `python3` is Python 3.13 and does not include a usable pip module. Use `uv` to create a Python 3.11 virtual environment with pip seeded into it.

### Clean local pip state

```bash
cd examples/firewall-pro-proxy/pypi
rm -rf .venv /tmp/fwpro-proxy-pypi-download
uv venv --seed --python 3.11 .venv
. .venv/bin/activate
python -m pip install --upgrade pip
```

### Pull regular dependencies through the proxy

```bash
python -m pip install --index-url https://proxy.wn.leyux.de/pypi/simple/ -r requirements.txt
```

Expected result: install succeeds. The sample `requirements.txt` intentionally contains a normal package version.

### Pull the allowed Sonatype sample through the proxy

```bash
python -m pip download --no-deps --dest /tmp/fwpro-proxy-pypi-download   --index-url https://proxy.wn.leyux.de/pypi/simple/ python-policy-demo==1.0.0
```

Expected result: download succeeds. Version `1.0.0` is the normal/allowed Sonatype sample.

### Try a malicious sample through the proxy

```bash
python -m pip download --no-deps --dest /tmp/fwpro-proxy-pypi-download   --index-url https://proxy.wn.leyux.de/pypi/simple/ python-policy-demo==1.1.0
```

Expected result: Firewall Pro hides/blocks the malicious version. pip should report `No matching distribution found` and show only the allowed version `1.0.0`.

Versions `1.1.0`, `1.2.0`, and `1.3.0` are Sonatype sample non-normal versions; `1.0.0` is normal and allowed.

### Presenter signal

After a fresh request, proxy logs on docker-wn should show PyPI artifact URLs under:

```text
https://firewall.sonatype.app/pypi/packages/
```

If logs show `https://files.pythonhosted.org/` for a fresh cache miss, the PyPI route is bypassing Firewall Pro and the demo should be paused.


## Maven demo

Example files live in:

```text
examples/firewall-pro-proxy/maven/pom.xml
examples/firewall-pro-proxy/maven/settings.xml
examples/firewall-pro-proxy/maven/settings.gradle.kts
```

The Maven demo uses an isolated local repository and the checked-in Maven mirror settings file so it does not depend on any user-level Maven settings.

### Clean local Maven state and force Maven through the proxy

Use an isolated local repository plus `examples/firewall-pro-proxy/maven/settings.xml`. The settings file contains `<mirrorOf>*</mirrorOf>` so every Maven repository request is mirrored to `https://proxy.wn.leyux.de/maven/`. This is important: `mvn dependency:get -DremoteRepositories=...` alone can still resolve from Maven Central in some environments, which hides whether Firewall Pro was actually used.

```bash
cd examples/firewall-pro-proxy/maven
repo=/tmp/fwpro-proxy-maven-demo/repo
rm -rf /tmp/fwpro-proxy-maven-demo
mkdir -p "$repo"
```

### Pull the allowed Sonatype sample through the proxy

Request the **JAR** (the actual component binary), not just the POM. Firewall Pro
only quarantines the component artifact; the `.pom` is metadata and is always served,
so a `:pom` request can never demonstrate a block.

```bash
mvn -q -s settings.xml \
  -Dmaven.repo.local="$repo" \
  -Dartifact=org.sonatype:maven-policy-demo:1.0.0:jar \
  dependency:get
```

Expected result: Maven exits successfully (`EXIT=0`) and writes the JAR under `$repo/org/sonatype/maven-policy-demo/1.0.0/`.

### Try the malicious Sonatype sample through the proxy

```bash
mvn -q -s settings.xml \
  -Dmaven.repo.local="$repo" \
  -Dartifact=org.sonatype:maven-policy-demo:1.1.0:jar \
  dependency:get
```

Expected result: the build **fails**. Firewall Pro returns `403` on the JAR with a
`Sonatype Firewall Report` body (`malicious_state=MALICIOUS, ri_state=SUSPICIOUS`). The
proxy forwards that `403` and the report body to the client, and Maven reports:

```text
Could not transfer artifact org.sonatype:maven-policy-demo:jar:1.1.0
from/to firewall-pro-proxy (https://proxy.wn.leyux.de/maven/): status code: 403,
reason phrase: Forbidden (403)
```

Presenter tip: `curl -s https://proxy.wn.leyux.de/maven/org/sonatype/maven-policy-demo/1.1.0/maven-policy-demo-1.1.0.jar`
returns the raw Sonatype Firewall Report JSON, which is a clean thing to show on screen.

Why the earlier `:pom` procedure did not show a block: Firewall Pro quarantines the
**component JAR**, not the POM. Fetching `...:1.1.0:pom` only pulls metadata, which is
always served (`200`), so it can never demonstrate blocking. Direct-to-Firewall probes
confirm this split — for 1.1.0/1.2.0/1.3.0 the `.pom` returns `200` while the `.jar`
returns `403`; for the allowed 1.0.0 the `.jar` returns `302` (redirect to the real
download). Always demo Maven blocking with a `:jar` (or a full `dependency:resolve` that
pulls the binary).

Known Sonatype Maven sample versions:

- `1.0.0` - Normal
- `1.1.0` - Suspicious; malicious Security Vulnerability Category
- `1.2.0` - Suspicious
- `1.3.0` - Pending

### Optional: use the sample pom.xml

The included `pom.xml` declares `https://proxy.wn.leyux.de/maven/` as its repository and
depends on the allowed sample version as a **JAR** (no `<type>pom</type>`), so
`dependency:resolve` pulls the component binary through Firewall Pro. The `settings.xml`
mirror still forces all plugin/dependency repository access through the proxy:

```bash
mvn -q -s settings.xml -Dmaven.repo.local="$repo" dependency:resolve
```

To show the block from a real build, bump the dependency version in `pom.xml` from
`1.0.0` to `1.1.0` and re-run `dependency:resolve`: the build fails with the same
Firewall `403` because Maven now has to fetch the quarantined JAR.

### Gradle plugin resolution note

The upstream `git-pkgs/proxy` README also documents Gradle plugin resolution through the Maven endpoint. A Gradle build should configure plugin repositories like this:

```kotlin
pluginManagement {
  repositories {
    maven(url = "https://proxy.wn.leyux.de/maven/")
  }
}
```

The example `settings.gradle.kts` in this directory contains that configuration.

Current live state: the proxy source intentionally disables Gradle Plugin Portal fallback for the Firewall Pro demo. With `upstream.maven` set to `https://firewall.sonatype.app/mvn/`, a fresh request for a Gradle plugin marker POM such as `com.diffplug.spotless:com.diffplug.spotless.gradle.plugin:8.4.0` should only try `https://firewall.sonatype.app/mvn/...` and return `404` if Firewall does not serve that marker. Do not present Gradle plugin resolution as part of the Firewall Pro demo.

The README's separate `/gradle/` endpoint is Gradle HTTP Build Cache, not Maven dependency or plugin resolution. It is unrelated to the Firewall Pro package-blocking demo.

### Presenter signal

After a fresh request, proxy logs on docker-wn should show Maven upstream URLs under:

```text
https://firewall.sonatype.app/mvn/
```

If logs show `https://repo1.maven.org/maven2/` for a fresh cache miss, the Maven route is bypassing Firewall Pro and the demo should be paused.
