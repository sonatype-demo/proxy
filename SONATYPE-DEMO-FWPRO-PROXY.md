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
| Maven | `https://proxy.wn.leyux.de/maven/` | `https://firewall.sonatype.app/mvn/` | traffic routes through Firewall Pro; malicious sample blocking not currently observed |

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
examples/firewall-pro-proxy/maven/
```

The Maven demo uses an isolated local repository and command-line repository override so it does not depend on any user-level Maven settings.

### Clean local Maven state

```bash
cd examples/firewall-pro-proxy/maven
rm -rf /tmp/fwpro-proxy-maven-repo
mkdir -p /tmp/fwpro-proxy-maven-repo
```

### Pull the allowed Sonatype sample through the proxy

```bash
mvn -q   -Dmaven.repo.local=/tmp/fwpro-proxy-maven-repo   -DremoteRepositories=firewall-pro-proxy::default::https://proxy.wn.leyux.de/maven/   -Dartifact=org.sonatype:maven-policy-demo:1.0.0:pom   dependency:get
```

Expected result: Maven exits successfully and writes the POM under `/tmp/fwpro-proxy-maven-repo/org/sonatype/maven-policy-demo/1.0.0/`.

### Try the non-normal Sonatype sample through the proxy

```bash
mvn -q   -Dmaven.repo.local=/tmp/fwpro-proxy-maven-repo   -DremoteRepositories=firewall-pro-proxy::default::https://proxy.wn.leyux.de/maven/   -Dartifact=org.sonatype:maven-policy-demo:1.1.0:pom   dependency:get
```

Current verified behavior: Maven traffic is routed through Firewall Pro, but `org.sonatype:maven-policy-demo:1.1.0:pom` still resolves/downloads. Therefore this Maven procedure demonstrates Firewall Pro as the upstream registry/cache path, but it must not be presented as proof that malicious Maven packages are blocked today.

Known Sonatype Maven sample versions:

- `1.0.0` - Normal
- `1.1.0` - Suspicious; malicious Security Vulnerability Category
- `1.2.0` - Suspicious
- `1.3.0` - Pending

### Optional: use the sample pom.xml

The included `pom.xml` declares `https://proxy.wn.leyux.de/maven/` as its repository and depends on the allowed sample version:

```bash
mvn -q -Dmaven.repo.local=/tmp/fwpro-proxy-maven-repo dependency:resolve
```

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

Current live check: this Gradle plugin-resolution path is not a working Firewall Pro demo yet. With `upstream.maven` set to `https://firewall.sonatype.app/mvn/`, a fresh request for a Gradle plugin marker POM such as `com.diffplug.spotless:com.diffplug.spotless.gradle.plugin:8.4.0` returned `502` through the proxy. Logs showed the proxy tried `https://firewall.sonatype.app/mvn/...`; the direct Firewall response for that marker is `404`, while `https://plugins.gradle.org/m2/...` serves the marker. Do not present Gradle plugin resolution as verified until the Maven handler fallback/auth behavior is fixed or the Gradle Plugin Portal path is intentionally configured and tested.

The README's separate `/gradle/` endpoint is Gradle HTTP Build Cache, not Maven dependency or plugin resolution. It is unrelated to the Firewall Pro package-blocking demo.

### Presenter signal

After a fresh request, proxy logs on docker-wn should show Maven upstream URLs under:

```text
https://firewall.sonatype.app/mvn/
```

If logs show `https://repo1.maven.org/maven2/` for a fresh cache miss, the Maven route is bypassing Firewall Pro and the demo should be paused.
