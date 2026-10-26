#!/usr/bin/env bash
set -euo pipefail

# Verify Linux DEB and RPM artifacts according to docs/distribution-plan.md
DIST_DIR="${1:-dist}"

if [ ! -d "$DIST_DIR" ]; then
  echo "Error: Directory '$DIST_DIR' does not exist." >&2
  exit 1
fi

command -v dpkg-deb >/dev/null 2>&1 || {
  echo "Error: dpkg-deb is required for package verification." >&2
  exit 1
}

command -v rpm >/dev/null 2>&1 || {
  echo "Error: rpm is required for package verification." >&2
  exit 1
}

EXPECTED_FILES=(
  "/usr/bin/podmutt"
  "/usr/share/applications/podmutt.desktop"
  "/usr/share/icons/hicolor/scalable/apps/podmutt.svg"
  "/usr/share/icons/hicolor/512x512/apps/podmutt.png"
  "/usr/share/doc/podmutt/copyright"
  "/usr/share/doc/podmutt/README.md"
)

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

HOST_ARCH="$(uname -m)"
case "$HOST_ARCH" in
  x86_64)
    HOST_DEB_ARCH="amd64"
    HOST_RPM_ARCH="x86_64"
    ;;
  aarch64|arm64)
    HOST_DEB_ARCH="arm64"
    HOST_RPM_ARCH="aarch64"
    ;;
  *)
    HOST_DEB_ARCH=""
    HOST_RPM_ARCH=""
    ;;
esac

echo "==> Verifying DEB and RPM packages in $DIST_DIR (Host: $HOST_ARCH)..."

# 1. Verify DEB Packages
DEB_FILES=($(find "$DIST_DIR" -maxdepth 1 -name "*.deb" | sort))
if [ ${#DEB_FILES[@]} -eq 0 ]; then
  echo "Error: No .deb files found in $DIST_DIR." >&2
  exit 1
fi

for deb in "${DEB_FILES[@]}"; do
  echo "--> Inspecting DEB: $(basename "$deb")"
  
  # Metadata
  pkg_name="$(dpkg-deb -f "$deb" Package)"
  pkg_arch="$(dpkg-deb -f "$deb" Architecture)"
  pkg_depends="$(dpkg-deb -f "$deb" Depends)"
  pkg_homepage="$(dpkg-deb -f "$deb" Homepage)"
  pkg_section="$(dpkg-deb -f "$deb" Section)"
  pkg_version="$(dpkg-deb -f "$deb" Version)"

  if [ "$pkg_name" != "podmutt" ]; then
    echo "FAIL: Expected Package 'podmutt', got '$pkg_name'" >&2
    exit 1
  fi

  if [[ "$pkg_arch" != "amd64" && "$pkg_arch" != "arm64" ]]; then
    echo "FAIL: Unexpected DEB architecture '$pkg_arch'" >&2
    exit 1
  fi

  if [[ "$pkg_depends" != *"mpv"* || "$pkg_depends" != *"ca-certificates"* ]]; then
    echo "FAIL: Expected mpv and ca-certificates dependencies, got '$pkg_depends'" >&2
    exit 1
  fi

  if [ "$pkg_homepage" != "https://github.com/susamn/podmutt" ]; then
    echo "FAIL: Unexpected Homepage '$pkg_homepage'" >&2
    exit 1
  fi

  if [ "$pkg_section" != "sound" ]; then
    echo "FAIL: Expected Section 'sound', got '$pkg_section'" >&2
    exit 1
  fi

  # Payload inspection
  contents="$(dpkg-deb -c "$deb")"
  for expected in "${EXPECTED_FILES[@]}"; do
    # dpkg-deb output lines end with .<expected>
    if ! echo "$contents" | grep -q "[[:space:]]\.${expected}$"; then
      echo "FAIL: Missing expected payload entry '.$expected' in $deb" >&2
      exit 1
    fi
  done

  # Check permissions: /usr/bin/podmutt must be -rwxr-xr-x
  bin_entry="$(echo "$contents" | grep "[[:space:]]\./usr/bin/podmutt$")"
  if [[ "$bin_entry" != -rwxr-xr-x* ]]; then
    echo "FAIL: /usr/bin/podmutt has invalid permissions: $bin_entry" >&2
    exit 1
  fi

  # Exclusions check: no macOS or Darwin artifacts
  if echo "$contents" | grep -Ei "\.icns|Info\.plist|\.DS_Store|darwin"; then
    echo "FAIL: Prohibited macOS/Darwin artifact detected in $deb" >&2
    exit 1
  fi

  # Extract binary for inspection & execution check
  deb_extract="$TMP_DIR/deb_extract_${pkg_arch}"
  mkdir -p "$deb_extract"
  dpkg-deb -x "$deb" "$deb_extract"
  extracted_bin="$deb_extract/usr/bin/podmutt"

  if [ ! -f "$extracted_bin" ]; then
    echo "FAIL: Extracted binary not found at $extracted_bin" >&2
    exit 1
  fi

  bin_file_type="$(file "$extracted_bin")"
  if [[ "$bin_file_type" != *"ELF 64-bit"* ]]; then
    echo "FAIL: Packaged binary is not an ELF 64-bit executable: $bin_file_type" >&2
    exit 1
  fi

  # Verify static linking (no dynamic dependencies)
  if [[ "$bin_file_type" != *"statically linked"* ]]; then
    echo "FAIL: Packaged binary is not statically linked: $bin_file_type" >&2
    exit 1
  fi

  # If host architecture matches, run smoke execution
  if [ "$pkg_arch" = "$HOST_DEB_ARCH" ]; then
    echo "    Running binary smoke tests for $pkg_arch..."
    v_out="$("$extracted_bin" --version)"
    if [[ "$v_out" != *"podmutt"* ]]; then
      echo "FAIL: --version output invalid: $v_out" >&2
      exit 1
    fi

    h_out="$("$extracted_bin" --help 2>&1)"
    if [[ "$h_out" != *"Usage: podmutt"* ]]; then
      echo "FAIL: --help output invalid: $h_out" >&2
      exit 1
    fi
    echo "    Smoke tests passed for $pkg_arch binary ($v_out)."
  else
    echo "    Cross-compiled architecture ($pkg_arch != $HOST_DEB_ARCH); skipping runtime execution on host."
  fi
done

# 2. Verify RPM Packages
RPM_FILES=($(find "$DIST_DIR" -maxdepth 1 -name "*.rpm" | sort))
if [ ${#RPM_FILES[@]} -eq 0 ]; then
  echo "Error: No .rpm files found in $DIST_DIR." >&2
  exit 1
fi

for rpm_pkg in "${RPM_FILES[@]}"; do
  echo "--> Inspecting RPM: $(basename "$rpm_pkg")"

  rpm_name="$(rpm -qp --qf '%{NAME}' "$rpm_pkg")"
  rpm_arch="$(rpm -qp --qf '%{ARCH}' "$rpm_pkg")"
  rpm_license="$(rpm -qp --qf '%{LICENSE}' "$rpm_pkg")"
  rpm_url="$(rpm -qp --qf '%{URL}' "$rpm_pkg")"
  rpm_reqs="$(rpm -qp --requires "$rpm_pkg")"

  if [ "$rpm_name" != "podmutt" ]; then
    echo "FAIL: Expected RPM Name 'podmutt', got '$rpm_name'" >&2
    exit 1
  fi

  if [[ "$rpm_arch" != "x86_64" && "$rpm_arch" != "aarch64" ]]; then
    echo "FAIL: Unexpected RPM architecture '$rpm_arch'" >&2
    exit 1
  fi

  if [ "$rpm_license" != "MIT" ]; then
    echo "FAIL: Expected License 'MIT', got '$rpm_license'" >&2
    exit 1
  fi

  if [ "$rpm_url" != "https://github.com/susamn/podmutt" ]; then
    echo "FAIL: Expected URL 'https://github.com/susamn/podmutt', got '$rpm_url'" >&2
    exit 1
  fi

  if [[ "$rpm_reqs" != *"mpv"* || "$rpm_reqs" != *"ca-certificates"* ]]; then
    echo "FAIL: Expected mpv and ca-certificates in RPM requires, got: $rpm_reqs" >&2
    exit 1
  fi

  # Payload inspection
  rpm_files="$(rpm -qlp "$rpm_pkg")"
  for expected in "${EXPECTED_FILES[@]}"; do
    if ! echo "$rpm_files" | grep -q "^${expected}$"; then
      echo "FAIL: Missing expected payload entry '$expected' in $rpm_pkg" >&2
      exit 1
    fi
  done

  # Exclusions check
  if echo "$rpm_files" | grep -Ei "\.icns|Info\.plist|\.DS_Store|darwin"; then
    echo "FAIL: Prohibited macOS/Darwin artifact detected in $rpm_pkg" >&2
    exit 1
  fi

  # Extract binary for inspection & execution check if rpm2cpio is present
  if command -v rpm2cpio >/dev/null 2>&1 && command -v cpio >/dev/null 2>&1; then
    rpm_extract="$TMP_DIR/rpm_extract_${rpm_arch}"
    mkdir -p "$rpm_extract"
    abs_rpm_pkg="$(cd "$(dirname "$rpm_pkg")" && pwd)/$(basename "$rpm_pkg")"
    (cd "$rpm_extract" && rpm2cpio "$abs_rpm_pkg" | cpio -idmv >/dev/null 2>&1)
    extracted_rpm_bin="$rpm_extract/usr/bin/podmutt"

    if [ ! -f "$extracted_rpm_bin" ]; then
      echo "FAIL: Extracted RPM binary not found at $extracted_rpm_bin" >&2
      exit 1
    fi

    bin_file_type="$(file "$extracted_rpm_bin")"
    if [[ "$bin_file_type" != *"ELF 64-bit"* ]]; then
      echo "FAIL: Packaged RPM binary is not an ELF 64-bit executable: $bin_file_type" >&2
      exit 1
    fi

    if [[ "$bin_file_type" != *"statically linked"* ]]; then
      echo "FAIL: Packaged RPM binary is not statically linked: $bin_file_type" >&2
      exit 1
    fi

    if [ "$rpm_arch" = "$HOST_RPM_ARCH" ]; then
      echo "    Running RPM binary smoke tests for $rpm_arch..."
      v_out="$("$extracted_rpm_bin" --version)"
      if [[ "$v_out" != *"podmutt"* ]]; then
        echo "FAIL: RPM --version output invalid: $v_out" >&2
        exit 1
      fi

      h_out="$("$extracted_rpm_bin" --help 2>&1)"
      if [[ "$h_out" != *"Usage: podmutt"* ]]; then
        echo "FAIL: RPM --help output invalid: $h_out" >&2
        exit 1
      fi
      echo "    Smoke tests passed for $rpm_arch RPM binary ($v_out)."
    else
      echo "    Cross-compiled architecture ($rpm_arch != $HOST_RPM_ARCH); skipping runtime execution on host."
    fi
  fi
done

echo "==> All DEB and RPM packages successfully verified against the canonical contract."
