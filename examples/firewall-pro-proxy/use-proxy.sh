#!/usr/bin/env bash

: "${PROXY_URL:?Set PROXY_URL to the proxy base URL first}"
PROXY_URL="${PROXY_URL%/}"

export NPM_UPSTREAM="$PROXY_URL/npm"
export PYPI_UPSTREAM="$PROXY_URL/pypi"
export MAVEN_UPSTREAM="$PROXY_URL/maven"
export NUGET_UPSTREAM="$PROXY_URL/nuget"
