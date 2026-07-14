#!/usr/bin/env bash
#
# verify-firewall-blocking.sh
#
# Verifies Sonatype Repository Firewall Pro policy behaviour for the Sonatype
# "policy-demo" sample packages across npm, PyPI, Maven and NuGet:
#
#   * known-malicious versions are BLOCKED, and
#   * known-good versions are NOT blocked.
#
# It talks DIRECTLY to Firewall Pro (default https://firewall.sonatype.app),
# bypassing the git-pkgs proxy, so a failure points at Firewall/policy config
# rather than the proxy. It is deliberately safe to run in CI: no package bytes
# are downloaded. It only reads registry metadata/index responses and inspects
# the HTTP status of artifact URLs; HTTP redirects are NOT followed, so a "302"
# (allowed -> redirect to the real CDN) is treated as served without ever
# fetching the artifact, and malicious bytes are never pulled.
#
# Blocking signal per ecosystem (direct from Firewall Pro):
#   npm    tarball  -> HTTP 403 blocked  /  200|302 served      (POM/metadata is
#   maven  jar      -> HTTP 403 blocked  /  200|302 served       never blocked;
#                                                                the JAR is)
#   pypi   version  -> absent from the PEP 503 simple index = blocked
#                      (Firewall omits the malicious versions' download anchors)
#
# Credentials are read from the environment and are never printed or placed on
# the process command line:
#   SONATYPE_FIREWALL_USERNAME
#   SONATYPE_FIREWALL_PASSWORD
#
# Usage:
#   set -a; . ./.env; set +a      # load the Firewall basic-auth creds (gitignored, this folder)
#   ./verify-firewall-blocking.sh
#
# Optional overrides (default to the public Firewall Pro host):
#   FIREWALL_BASE   e.g. https://firewall.sonatype.app
#   NPM_UPSTREAM    default $FIREWALL_BASE/npm
#   PYPI_UPSTREAM   default $FIREWALL_BASE/pypi
#   MAVEN_UPSTREAM  default $FIREWALL_BASE/mvn
#   NUGET_UPSTREAM  default $FIREWALL_BASE/nuget
#
# Exit status: 0 if every expectation holds, 1 if any check fails,
#              2 on a setup error (missing creds / curl).

set -uo pipefail

FIREWALL_BASE="${FIREWALL_BASE:-https://firewall.sonatype.app}"
NPM_UPSTREAM="${NPM_UPSTREAM:-$FIREWALL_BASE/npm}"
PYPI_UPSTREAM="${PYPI_UPSTREAM:-$FIREWALL_BASE/pypi}"
MAVEN_UPSTREAM="${MAVEN_UPSTREAM:-$FIREWALL_BASE/mvn}"
NUGET_UPSTREAM="${NUGET_UPSTREAM:-$FIREWALL_BASE/nuget}"

DIRECT_FIREWALL_BASE="https://firewall.sonatype.app"
if [ "${FIREWALL_BASE%/}" = "$DIRECT_FIREWALL_BASE" ] \
  && [ "${NPM_UPSTREAM%/}" = "$DIRECT_FIREWALL_BASE/npm" ] \
  && [ "${PYPI_UPSTREAM%/}" = "$DIRECT_FIREWALL_BASE/pypi" ] \
  && [ "${MAVEN_UPSTREAM%/}" = "$DIRECT_FIREWALL_BASE/mvn" ] \
  && [ "${NUGET_UPSTREAM%/}" = "$DIRECT_FIREWALL_BASE/nuget" ]; then
  CONNECTION_MODE="direct"
else
  CONNECTION_MODE="non-direct"
fi

# --- Sonatype policy-demo sample coordinates --------------------------------
NPM_PKG="@sonatype/policy-demo"
NPM_ALLOWED=(2.0.0)
NPM_MALICIOUS=(2.1.0 2.2.0 2.3.0)

PYPI_PKG="python-policy-demo"
PYPI_ALLOWED=(1.0.0)
PYPI_MALICIOUS=(1.1.0 1.2.0 1.3.0)

MVN_GROUP_PATH="org/sonatype/maven-policy-demo"
MVN_ARTIFACT="maven-policy-demo"
MVN_ALLOWED=(1.0.0)
MVN_MALICIOUS=(1.1.0 1.2.0 1.3.0)

NUGET_PKG="sonatype.sonatype-policy-demo.package"   # lower-case id (flatcontainer path)
NUGET_ALLOWED=(1.0.0)
NUGET_MALICIOUS=(1.1.0 1.2.0 1.3.0)

# --- preflight --------------------------------------------------------------
if [ -z "${SONATYPE_FIREWALL_USERNAME:-}" ] || [ -z "${SONATYPE_FIREWALL_PASSWORD:-}" ]; then
  echo "error: set SONATYPE_FIREWALL_USERNAME and SONATYPE_FIREWALL_PASSWORD" >&2
  echo "       e.g.  set -a; . ./.env; set +a" >&2
  exit 2
fi
command -v curl >/dev/null 2>&1 || { echo "error: curl is required" >&2; exit 2; }

# Keep credentials out of the argument list (ps) and out of the terminal by
# feeding them to curl through a 0600 config file that forces preemptive Basic.
CURL_CFG="$(mktemp)"
chmod 600 "$CURL_CFG"
trap 'rm -f "$CURL_CFG"' EXIT
printf 'user = "%s:%s"\n' "$SONATYPE_FIREWALL_USERNAME" "$SONATYPE_FIREWALL_PASSWORD" > "$CURL_CFG"

CURL=(curl -sS -K "$CURL_CFG" --max-time 30)

pass=0
fail=0

report() { # $1=PASS|FAIL  $2=message
  if [ "$1" = PASS ]; then
    pass=$((pass + 1)); printf '  [PASS] %s\n' "$2"
  else
    fail=$((fail + 1)); printf '  [FAIL] %s\n' "$2"
  fi
}

status_of() { # $1=url -> echoes final HTTP status (no redirect following, no body)
  "${CURL[@]}" -o /dev/null -w '%{http_code}' "$1"
}

# npm/Maven block with 403, NuGet with 409; any 2xx/3xx means the artifact is served.
check_artifact() { # $1=label $2=url $3=expected(allowed|blocked)
  local label="$1" url="$2" expected="$3" code outcome
  code="$(status_of "$url")"
  case "$code" in
    403|409)                outcome=blocked ;;
    200|301|302|303|307|308) outcome=allowed ;;
    *) report FAIL "$label  expected=$expected but got unexpected HTTP ${code:-<none>}"; return ;;
  esac
  if [ "$outcome" = "$expected" ]; then
    report PASS "$label  expected=$expected  observed=$outcome (HTTP $code)"
  else
    report FAIL "$label  expected=$expected  observed=$outcome (HTTP $code)"
  fi
}

echo "Sonatype Firewall Pro blocking verification"
if [ "$CONNECTION_MODE" = direct ]; then
  echo "Connection mode: DIRECT to $DIRECT_FIREWALL_BASE"
else
  echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
  echo "!! NON-DIRECT MODE: not talking directly to"
  echo "!! $DIRECT_FIREWALL_BASE"
  echo "!! Requests are using proxy or overridden ecosystem endpoints."
  echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
  echo "Effective endpoints:"
  echo "  npm:    $NPM_UPSTREAM"
  echo "  PyPI:   $PYPI_UPSTREAM"
  echo "  Maven:  $MAVEN_UPSTREAM"
  echo "  NuGet:  $NUGET_UPSTREAM"
fi
echo

echo "== npm  $NPM_PKG  ($NPM_UPSTREAM) =="
npm_name="${NPM_PKG##*/}"   # scoped tarball path: @scope/name/-/name-version.tgz
for v in "${NPM_ALLOWED[@]}";   do check_artifact "npm   $NPM_PKG@$v"   "$NPM_UPSTREAM/$NPM_PKG/-/$npm_name-$v.tgz" allowed; done
for v in "${NPM_MALICIOUS[@]}"; do check_artifact "npm   $NPM_PKG@$v"   "$NPM_UPSTREAM/$NPM_PKG/-/$npm_name-$v.tgz" blocked; done
echo

echo "== Maven  org.sonatype:$MVN_ARTIFACT  ($MAVEN_UPSTREAM) =="
for v in "${MVN_ALLOWED[@]}";   do check_artifact "maven $MVN_ARTIFACT:$v (jar)" "$MAVEN_UPSTREAM/$MVN_GROUP_PATH/$v/$MVN_ARTIFACT-$v.jar" allowed; done
for v in "${MVN_MALICIOUS[@]}"; do check_artifact "maven $MVN_ARTIFACT:$v (jar)" "$MAVEN_UPSTREAM/$MVN_GROUP_PATH/$v/$MVN_ARTIFACT-$v.jar" blocked; done
echo

echo "== NuGet  $NUGET_PKG  ($NUGET_UPSTREAM) =="
nuget_url() { echo "$NUGET_UPSTREAM/v3-flatcontainer/$NUGET_PKG/$1/$NUGET_PKG.$1.nupkg"; }
for v in "${NUGET_ALLOWED[@]}";   do check_artifact "nuget $NUGET_PKG $v (nupkg)" "$(nuget_url "$v")" allowed; done
for v in "${NUGET_MALICIOUS[@]}"; do check_artifact "nuget $NUGET_PKG $v (nupkg)" "$(nuget_url "$v")" blocked; done
echo

echo "== PyPI  $PYPI_PKG  ($PYPI_UPSTREAM) =="
pypi_index="$("${CURL[@]}" "$PYPI_UPSTREAM/simple/$PYPI_PKG/")"
if [ -z "$pypi_index" ]; then
  report FAIL "pypi  simple index fetch returned no body for $PYPI_PKG"
else
  # Firewall hides a blocked version by omitting its <a href> download anchor.
  pypi_hrefs="$(printf '%s' "$pypi_index" | grep -oE 'href="[^"]+"')"
  check_pypi() { # $1=version $2=expected
    local v="$1" expected="$2" vq outcome
    vq="${v//./\\.}"   # escape dots so the version token matches literally
    if printf '%s' "$pypi_hrefs" | grep -Eq "policy.demo-${vq}[-.#]"; then
      outcome=allowed
    else
      outcome=blocked
    fi
    if [ "$outcome" = "$expected" ]; then
      report PASS "pypi  $PYPI_PKG==$v  expected=$expected  observed=$outcome (simple index)"
    else
      report FAIL "pypi  $PYPI_PKG==$v  expected=$expected  observed=$outcome (simple index)"
    fi
  }
  for v in "${PYPI_ALLOWED[@]}";   do check_pypi "$v" allowed; done
  for v in "${PYPI_MALICIOUS[@]}"; do check_pypi "$v" blocked; done
fi
echo

echo "Summary: $pass passed, $fail failed."
[ "$fail" -eq 0 ] || exit 1
