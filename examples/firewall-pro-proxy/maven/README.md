# Maven — Firewall Pro proxy demo

Shows Sonatype Firewall Pro blocking a malicious Maven component while an allowed version
resolves normally, all through `git-pkgs proxy`.

- **Repository (proxy):** `https://proxy.wn.leyux.de/maven/`  → upstream `https://firewall.sonatype.app/mvn/`
- **Component:** `org.sonatype:maven-policy-demo`
- **Allowed:** `1.0.0`  **Blocked:** `1.1.0`, `1.2.0`, `1.3.0`

Sample version labels: `1.0.0` Normal · `1.1.0` Suspicious (malicious security-vulnerability
category) · `1.2.0` Suspicious · `1.3.0` Pending.

## Files in this folder

| File                  | Purpose                                                                    |
| --------------------- | -------------------------------------------------------------------------- |
| `settings.xml`        | Maven mirror (`<mirrorOf>*</mirrorOf>`) forcing **all** resolution through the proxy |
| `pom.xml`             | Minimal project depending on the allowed sample **JAR**                    |
| `settings.gradle.kts` | Gradle plugin-repo + HTTP build-cache config (see "Gradle note" below)     |

## Important: demo the block on the JAR, not the POM

Firewall Pro quarantines the **component JAR** (the binary), **not the POM**. The POM is only
metadata and is always served (`200`). A `:pom` request therefore can *never* show a block —
this is the trap the original demo fell into. Direct-to-Firewall probes make the split clear:

| Version | `.pom` | `.jar`                                   |
| ------- | ------ | ---------------------------------------- |
| `1.0.0` | `200`  | `302` (redirect to the real download = allowed) |
| `1.1.0` | `200`  | `403` blocked                            |
| `1.2.0` | `200`  | `403` blocked                            |
| `1.3.0` | `200`  | `403` blocked                            |

**Always demo Maven blocking with the `:jar`** (or a `dependency:resolve` that pulls the
binary).

The `settings.xml` mirror (`<mirrorOf>*</mirrorOf>`) is required: `mvn dependency:get
-DremoteRepositories=...` alone can still resolve from Maven Central in some environments,
which would hide whether Firewall Pro was used at all.

## 1. Clean, isolated local repository

```bash
cd examples/firewall-pro-proxy/maven
repo=/tmp/fwpro-proxy-maven-demo/repo
rm -rf /tmp/fwpro-proxy-maven-demo
mkdir -p "$repo"
```

## 2. Pull the allowed sample JAR (succeeds)

```bash
mvn -q -s settings.xml \
  -Dmaven.repo.local="$repo" \
  -Dartifact=org.sonatype:maven-policy-demo:1.0.0:jar \
  dependency:get
```

Expected: exits `0` and writes the JAR under `$repo/org/sonatype/maven-policy-demo/1.0.0/`.

## 3. Try the malicious sample JAR (blocked)

```bash
mvn -q -s settings.xml \
  -Dmaven.repo.local="$repo" \
  -Dartifact=org.sonatype:maven-policy-demo:1.1.0:jar \
  dependency:get
```

Expected: the build **fails**. Firewall Pro returns `403` on the JAR with a *Sonatype
Firewall Report* body (`malicious_state=MALICIOUS, ri_state=SUSPICIOUS`); the proxy forwards
that `403` and Maven reports:

```text
Could not transfer artifact org.sonatype:maven-policy-demo:jar:1.1.0
from/to firewall-pro-proxy (https://proxy.wn.leyux.de/maven/): status code: 403,
reason phrase: Forbidden (403)
```

Show the raw block on screen (returns the report JSON):

```bash
curl -s https://proxy.wn.leyux.de/maven/org/sonatype/maven-policy-demo/1.1.0/maven-policy-demo-1.1.0.jar
```

> Before the proxy was fixed this surfaced as an opaque `502 Bad Gateway`; it now forwards
> Firewall's real `403` (see the top-level runbook, "What this fork changes").

## Optional: use the sample `pom.xml`

`pom.xml` depends on the allowed sample as a **JAR** (no `<type>pom</type>`), so
`dependency:resolve` pulls the component binary through Firewall Pro:

```bash
mvn -q -s settings.xml -Dmaven.repo.local="$repo" dependency:resolve
```

To show the block from a real build, bump the dependency version in `pom.xml` from `1.0.0` to
`1.1.0` and re-run — the build fails with the same Firewall `403`.

## Presenter signal

On a fresh request the proxy logs on docker-wn should show the Firewall upstream, and a block
is logged explicitly:

```bash
docker logs --since 2m git-pkgs-proxy 2>&1 | grep -E 'firewall.sonatype.app/mvn|blocked by upstream policy'
```

If a fresh cache miss instead shows `https://repo1.maven.org/maven2/`, the Maven route is
bypassing Firewall Pro — pause the demo.

## Gradle note (not part of the blocking demo)

`settings.gradle.kts` documents two Gradle items from the upstream `git-pkgs/proxy` README:

1. **Plugin resolution** through the Maven endpoint (`pluginManagement { repositories {
   maven(url = "https://proxy.wn.leyux.de/maven/") } }`). Gradle Plugin Portal fallback is
   intentionally disabled in this fork for the Firewall demo, so a plugin marker resolves
   only if Firewall serves it (else `404`). Do not present Gradle plugin resolution as part of
   the blocking demo.
2. The separate `/gradle/` **HTTP Build Cache** endpoint — unrelated to package resolution or
   Firewall blocking.
