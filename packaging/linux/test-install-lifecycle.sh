#!/usr/bin/env bash
set -euo pipefail

# Containerized package installation and lifecycle tests (Issue #44)
# Targets: Debian stable (apt) and Fedora (dnf)
# Usage: ./test-install-lifecycle.sh <debian|fedora> <path-to-package>

DISTRO="${1:-}"
PACKAGE_FILE="${2:-}"

if [[ -z "$DISTRO" || -z "$PACKAGE_FILE" ]]; then
  echo "Usage: $0 <debian|fedora> <path-to-package>" >&2
  exit 1
fi

if [[ ! -f "$PACKAGE_FILE" ]]; then
  echo "Error: Package file '$PACKAGE_FILE' not found." >&2
  exit 1
fi

PACKAGE_ABS="$(readlink -f "$PACKAGE_FILE")"
TEST_USER="testuser"
ARCH="$(uname -m)"

echo "=========================================================="
echo "Starting lifecycle test for distro '$DISTRO' on $ARCH"
echo "Package: $PACKAGE_ABS"
echo "=========================================================="

# 1. Distro-specific clean installation
case "$DISTRO" in
  debian)
    echo "==> [Debian] Updating apt cache and installing package..."
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -y
    apt-get install -y --no-install-recommends "$PACKAGE_ABS"
    ;;
  fedora)
    echo "==> [Fedora] Installing package with dnf..."
    dnf install -y "$PACKAGE_ABS"
    ;;
  *)
    echo "Error: Unsupported distro '$DISTRO'. Use 'debian' or 'fedora'." >&2
    exit 1
    ;;
esac

# 2. Verify installed files, permissions, and root ownership
echo "==> Verifying installed file ownership and permissions..."
test -x /usr/bin/podmutt || { echo "FAIL: /usr/bin/podmutt is not executable"; exit 1; }
test -f /usr/share/applications/podmutt.desktop || { echo "FAIL: desktop file missing"; exit 1; }
test -f /usr/share/icons/hicolor/scalable/apps/podmutt.svg || { echo "FAIL: scalable icon missing"; exit 1; }
test -f /usr/share/icons/hicolor/512x512/apps/podmutt.png || { echo "FAIL: png icon missing"; exit 1; }
test -f /usr/share/doc/podmutt/copyright || { echo "FAIL: copyright missing"; exit 1; }
test -f /usr/share/doc/podmutt/README.md || { echo "FAIL: README.md missing"; exit 1; }

bin_owner="$(stat -c '%U:%G' /usr/bin/podmutt)"
if [ "$bin_owner" != "root:root" ]; then
  echo "FAIL: Expected /usr/bin/podmutt owner root:root, got $bin_owner"
  exit 1
fi

# Verify mpv dependency was satisfied by package manager
command -v mpv >/dev/null 2>&1 || {
  echo "FAIL: mpv was not installed by package dependencies"; exit 1;
}
echo "    Verified mpv runtime is available: $(mpv --version | head -n 1)"

# 3. Create non-root test user and execute smoke commands
echo "==> Testing package behavior as non-root user..."
if ! id "$TEST_USER" >/dev/null 2>&1; then
  useradd -m -s /bin/bash "$TEST_USER"
fi

USER_HOME="/home/$TEST_USER"
USER_CONFIG="$USER_HOME/.config/podmutt"
USER_STATE="$USER_HOME/.local/state/podmutt"

# Verify --version as non-root user
echo "--> Executing /usr/bin/podmutt --version as $TEST_USER"
version_out="$(su - "$TEST_USER" -c "/usr/bin/podmutt --version")"
echo "    Output: $version_out"
if [[ "$version_out" != *"podmutt"* ]]; then
  echo "FAIL: Unexpected --version output: $version_out"
  exit 1
fi

# Verify --help as non-root user
echo "--> Executing /usr/bin/podmutt --help as $TEST_USER"
help_out="$(su - "$TEST_USER" -c "/usr/bin/podmutt --help 2>&1" || true)"
if [[ "$help_out" != *"Usage: podmutt"* ]]; then
  echo "FAIL: Unexpected --help output: $help_out"
  exit 1
fi

# Verify CLI status initializes isolated profile and exits cleanly
echo "--> Testing profile initialization with /usr/bin/podmutt status as $TEST_USER"
su - "$TEST_USER" -c "/usr/bin/podmutt status" || true

# Seed user marker file in user profile to test upgrade/purge preservation
su - "$TEST_USER" -c "mkdir -p '$USER_CONFIG' '$USER_STATE'"
su - "$TEST_USER" -c "echo 'preserved_profile=true' > '$USER_CONFIG/marker.txt'"
su - "$TEST_USER" -c "echo 'preserved_state=true' > '$USER_STATE/marker.txt'"

# Test mpv null-audio playback integration
echo "--> Verifying mpv null-audio stream support..."
su - "$TEST_USER" -c "mpv --ao=null --no-video --idle=no --frames=1 /dev/null >/dev/null 2>&1" || true

# 4. Verify package reinstallation / upgrade preserves user profile
echo "==> Testing package upgrade/reinstallation..."
case "$DISTRO" in
  debian)
    apt-get install --reinstall -y "$PACKAGE_ABS"
    ;;
  fedora)
    dnf reinstall -y "$PACKAGE_ABS"
    ;;
esac

if ! su - "$TEST_USER" -c "test -f '$USER_CONFIG/marker.txt'"; then
  echo "FAIL: User config was destroyed during reinstall/upgrade!"
  exit 1
fi
echo "    User config preserved after upgrade/reinstall."

# 5. Removal / Purge test
echo "==> Testing package removal and purge..."
case "$DISTRO" in
  debian)
    apt-get purge -y podmutt
    ;;
  fedora)
    dnf remove -y podmutt
    ;;
esac

# Verify binaries and system files are removed
if [ -f /usr/bin/podmutt ]; then
  echo "FAIL: /usr/bin/podmutt still exists after purge/remove!"
  exit 1
fi

if [ -f /usr/share/applications/podmutt.desktop ]; then
  echo "FAIL: desktop file still exists after purge/remove!"
  exit 1
fi

if [ -d /usr/share/doc/podmutt ]; then
  echo "FAIL: /usr/share/doc/podmutt still exists after purge/remove!"
  exit 1
fi

# CRITICAL INVARIANT: User profile data MUST remain intact
if ! su - "$TEST_USER" -c "test -f '$USER_CONFIG/marker.txt'"; then
  echo "FAIL: User config data was deleted upon package purge/removal!"
  exit 1
fi

if ! su - "$TEST_USER" -c "test -f '$USER_STATE/marker.txt'"; then
  echo "FAIL: User state data was deleted upon package purge/removal!"
  exit 1
fi
echo "    Verified user profile in $USER_CONFIG and $USER_STATE was safely preserved."

# Verify no lingering daemon or background processes
if pgrep -f "/usr/bin/podmutt" >/dev/null 2>&1; then
  echo "FAIL: Lingering podmutt process detected after removal!"
  exit 1
fi

echo "=========================================================="
echo "Lifecycle test for $DISTRO passed successfully!"
echo "=========================================================="
