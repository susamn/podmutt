#!/usr/bin/env bash
set -euo pipefail

# Updates susamn/homebrew-tap Casks/podmutt.rb independently after a release is published.
TAG="${1:-}"

if [ -z "$TAG" ]; then
  echo "Error: Tag name required (e.g. v1.4.0)." >&2
  exit 1
fi

VERSION="${TAG#v}"

if [ -z "${HOMEBREW_TAP_TOKEN:-}" ]; then
  echo "::warning ::HOMEBREW_TAP_TOKEN is not set. Downstream Homebrew tap update skipped."
  exit 0
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "==> Fetching checksums.txt for release $TAG..."
gh release download "$TAG" -p checksums.txt -D "$TMP_DIR"

DARWIN_ARM64_SHA="$(grep -E "podmutt_.*_darwin_arm64\.tar\.gz" "$TMP_DIR/checksums.txt" | awk '{print $1}')"
DARWIN_AMD64_SHA="$(grep -E "podmutt_.*_darwin_amd64\.tar\.gz" "$TMP_DIR/checksums.txt" | awk '{print $1}')"
LINUX_ARM64_SHA="$(grep -E "podmutt_.*_linux_arm64\.tar\.gz" "$TMP_DIR/checksums.txt" | awk '{print $1}')"
LINUX_AMD64_SHA="$(grep -E "podmutt_.*_linux_amd64\.tar\.gz" "$TMP_DIR/checksums.txt" | awk '{print $1}')"

if [[ -z "$DARWIN_ARM64_SHA" || -z "$DARWIN_AMD64_SHA" || -z "$LINUX_ARM64_SHA" || -z "$LINUX_AMD64_SHA" ]]; then
  echo "Error: Could not locate all required architecture SHA256 checksums in checksums.txt." >&2
  exit 1
fi

CASK_FILE="$TMP_DIR/podmutt.rb"
cat <<EOF > "$CASK_FILE"
# This file was generated automatically by podmutt release automation. DO NOT EDIT.
cask "podmutt" do
  version "$VERSION"

  on_macos do
    on_arm do
      sha256 "$DARWIN_ARM64_SHA"
      url "https://github.com/susamn/podmutt/releases/download/v#{version}/podmutt_#{version}_darwin_arm64.tar.gz"
    end
    on_intel do
      sha256 "$DARWIN_AMD64_SHA"
      url "https://github.com/susamn/podmutt/releases/download/v#{version}/podmutt_#{version}_darwin_amd64.tar.gz"
    end
  end
  on_linux do
    on_arm do
      sha256 "$LINUX_ARM64_SHA"
      url "https://github.com/susamn/podmutt/releases/download/v#{version}/podmutt_#{version}_linux_arm64.tar.gz"
    end
    on_intel do
      sha256 "$LINUX_AMD64_SHA"
      url "https://github.com/susamn/podmutt/releases/download/v#{version}/podmutt_#{version}_linux_amd64.tar.gz"
    end
  end

  name "podmutt"
  desc "Terminal podcast client inspired by lazygit and mpdtui"
  homepage "https://github.com/susamn/podmutt"

  livecheck do
    skip "Auto-generated on release."
  end

  binary "podmutt"

  caveats <<~EOS
    podmutt streams podcast audio through mpv:

      brew install mpv

    macOS first launch:
      If macOS warns that the developer cannot be verified, allow it with:
        xattr -d com.apple.quarantine \$(which podmutt)
      (or click "Open Anyway" in System Settings > Privacy & Security).

    Icons use standard Unicode by default (no patched font required).
    To use Nerd Fonts, set \`icons = "nerd"\` in config.toml.
  EOS
end
EOF

echo "==> Updating susamn/homebrew-tap repository..."
TAP_DIR="$TMP_DIR/tap"
git clone --depth 1 "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/susamn/homebrew-tap.git" "$TAP_DIR"
mkdir -p "$TAP_DIR/Casks"
cp "$CASK_FILE" "$TAP_DIR/Casks/podmutt.rb"

cd "$TAP_DIR"
git config user.name "github-actions[bot]"
git config user.email "github-actions[bot]@users.noreply.github.com"

if git diff --quiet Casks/podmutt.rb; then
  echo "Casks/podmutt.rb is already up to date for version $VERSION."
  exit 0
fi

git add Casks/podmutt.rb
git commit -m "podmutt $TAG"
git push origin main
echo "==> Successfully updated susamn/homebrew-tap to $TAG."
