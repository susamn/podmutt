#!/usr/bin/env bash
set -euo pipefail

# Runner for Debian and Fedora container install lifecycle tests (Issue #44)
# Usage: ./packaging/linux/run-container-lifecycle.sh [debian|fedora|all]

TARGET="${1:-all}"
DIST_DIR="$(pwd)/dist"

CONTAINER_CMD=""
if command -v podman >/dev/null 2>&1; then
  CONTAINER_CMD="podman"
elif command -v docker >/dev/null 2>&1; then
  CONTAINER_CMD="docker"
fi

if [ -z "$CONTAINER_CMD" ]; then
  echo "Notice: Neither podman nor docker was found on the host."
  echo "To run container lifecycle tests locally, install podman or docker, or execute"
  echo "'packaging/linux/test-install-lifecycle.sh <distro> <package>' directly within a container."
  exit 0
fi

ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)
    DEB_ARCH="amd64"
    RPM_ARCH="x86_64"
    ;;
  aarch64|arm64)
    DEB_ARCH="arm64"
    RPM_ARCH="aarch64"
    ;;
  *)
    echo "Error: Unsupported architecture $ARCH for container lifecycle testing." >&2
    exit 1
    ;;
esac

run_debian() {
  local deb_file
  deb_file="$(find "$DIST_DIR" -maxdepth 1 -name "*_${DEB_ARCH}.deb" | head -n 1)"
  if [[ -z "$deb_file" || ! -f "$deb_file" ]]; then
    echo "Error: No .deb package found for $DEB_ARCH in $DIST_DIR. Run goreleaser snapshot first." >&2
    exit 1
  fi
  echo "==> Running Debian lifecycle test on $(basename "$deb_file") using $CONTAINER_CMD..."
  $CONTAINER_CMD run --rm \
    -v "$(pwd)":/workspace:ro \
    -v "$deb_file":/package.deb:ro \
    debian:bookworm-slim \
    /workspace/packaging/linux/test-install-lifecycle.sh debian /package.deb
}

run_fedora() {
  local rpm_file
  rpm_file="$(find "$DIST_DIR" -maxdepth 1 -name "*${RPM_ARCH}.rpm" | head -n 1)"
  if [[ -z "$rpm_file" || ! -f "$rpm_file" ]]; then
    echo "Error: No .rpm package found for $RPM_ARCH in $DIST_DIR. Run goreleaser snapshot first." >&2
    exit 1
  fi
  echo "==> Running Fedora lifecycle test on $(basename "$rpm_file") using $CONTAINER_CMD..."
  $CONTAINER_CMD run --rm \
    -v "$(pwd)":/workspace:ro \
    -v "$rpm_file":/package.rpm:ro \
    fedora:latest \
    /workspace/packaging/linux/test-install-lifecycle.sh fedora /package.rpm
}

case "$TARGET" in
  debian)
    run_debian
    ;;
  fedora)
    run_fedora
    ;;
  all)
    run_debian
    run_fedora
    ;;
  *)
    echo "Usage: $0 [debian|fedora|all]" >&2
    exit 1
    ;;
esac

echo "==> Container lifecycle verification completed successfully."
