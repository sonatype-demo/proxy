# PyPI / pip — Firewall Pro proxy demo

Shows Sonatype Firewall Pro blocking a malicious PyPI package while an allowed version
downloads normally, all through `git-pkgs proxy`.

- **Index (proxy):** `$PROXY_URL/pypi/simple/`  → upstream `https://firewall.sonatype.app/pypi/`
- **Package:** `python-policy-demo`
- **Allowed:** `1.0.0`  **Blocked:** `1.1.0`, `1.2.0`, `1.3.0`

Set the proxy URL first (default is the docker-wn deployment; use your own, e.g. a local
container `http://localhost:8080`):

```bash
export PROXY_URL=https://proxy.wn.leyux.de     # or: set -a; . ../.env; set +a
```

`requirements.txt` in this folder pins a normal third-party package (`requests`) for the
"regular dependency resolves through the proxy" step.

## How the block works

For PyPI, Firewall Pro **omits the malicious versions from the PEP 503 simple index** — the
index served for `python-policy-demo` contains download links only for the allowed `1.0.0`.
pip therefore reports `No matching distribution found` for a blocked version: there is simply
no file to fetch. (This is metadata-level blocking, so it does not depend on the proxy's
`403` forwarding.)

## Python on docker-wn

The host `python3` is 3.13 and has no usable `pip` module. Use `uv` to create a Python 3.11
venv with pip seeded in:

## 1. Clean state and create the venv

```bash
cd examples/firewall-pro-proxy/pypi
rm -rf .venv /tmp/fwpro-proxy-pypi-download
uv venv --seed --python 3.11 .venv
. .venv/bin/activate
python -m pip install --upgrade pip
```

## 2. Resolve a regular dependency through the proxy (succeeds)

```bash
python -m pip install --index-url $PROXY_URL/pypi/simple/ -r requirements.txt
```

Expected: install succeeds — proves normal packages flow through the proxy.

## 3. Download the allowed sample (succeeds)

```bash
python -m pip download --no-deps --dest /tmp/fwpro-proxy-pypi-download \
  --index-url $PROXY_URL/pypi/simple/ python-policy-demo==1.0.0
```

Expected: download succeeds. `1.0.0` is the normal, allowed sample.

## 4. Try a malicious sample (blocked)

```bash
python -m pip download --no-deps --dest /tmp/fwpro-proxy-pypi-download \
  --index-url $PROXY_URL/pypi/simple/ python-policy-demo==1.1.0
```

Expected: pip reports `No matching distribution found for python-policy-demo==1.1.0` — the
malicious version is hidden from the index.

Show it directly: the simple index lists only `1.0.0` download links:

```bash
curl -s $PROXY_URL/pypi/simple/python-policy-demo/ | grep -oE 'href="[^"]+"'
```

## Presenter signal

On a fresh request the proxy logs on docker-wn should show the Firewall upstream:

```bash
docker logs --since 2m git-pkgs-proxy 2>&1 | grep -E 'firewall.sonatype.app/pypi'
```

If a fresh cache miss instead shows `https://files.pythonhosted.org/`, the PyPI route is
bypassing Firewall Pro — pause the demo.
