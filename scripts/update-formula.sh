#!/usr/bin/env bash
# Render the cross-platform Homebrew formula and push it to the tap repo.
#
# A Homebrew *formula* (not a cask) is the only artifact that serves both macOS
# and Linux, so we ship one. We render it ourselves rather than via GoReleaser's
# `brews:` because that key is deprecated (removed in GoReleaser v2.16); a
# hand-rendered formula depends only on the stable Homebrew DSL.
#
# Runs in CI right after `goreleaser release`, reading sha256s from the local
# dist/checksums.txt (no release-asset round-trip, so no upload race).
#
# Env:
#   VERSION             release tag, e.g. v0.3.0            (required)
#   HOMEBREW_TAP_TOKEN  PAT with contents:write on the tap  (omit -> dry run:
#                       prints the formula and exits without pushing)
set -euo pipefail

REPO="agentic-wiki/wiki"
TAP_REPO="agentic-wiki/homebrew-tap"
FORMULA="wiki"
CHECKSUMS="dist/checksums.txt"

: "${VERSION:?set VERSION to the release tag, e.g. v0.3.0}"
version="${VERSION#v}" # formula version carries no leading "v"

[[ -f "$CHECKSUMS" ]] || {
    echo "ERROR: $CHECKSUMS not found — run 'goreleaser release' (or 'just release-dry') first" >&2
    exit 1
}

# sha256 for an archive basename, by exact match in checksums.txt ("<sha>  <name>").
sha_for() {
    local name="$1" sha
    sha=$(awk -v n="$name" '$2 == n {print $1}' "$CHECKSUMS")
    [[ -n "$sha" ]] || {
        echo "ERROR: $name not found in $CHECKSUMS" >&2
        exit 1
    }
    printf '%s' "$sha"
}

base="https://github.com/${REPO}/releases/download/${VERSION}"
darwin_arm="${FORMULA}_darwin_arm64.tar.gz"
darwin_amd="${FORMULA}_darwin_amd64.tar.gz"
linux_arm="${FORMULA}_linux_arm64.tar.gz"
linux_amd="${FORMULA}_linux_amd64.tar.gz"

formula=$(
    cat <<EOF
class Wiki < Formula
  desc "Query and maintain agentic-wiki bundles (OKF markdown)"
  homepage "https://github.com/${REPO}"
  version "${version}"

  on_macos do
    on_arm do
      url "${base}/${darwin_arm}"
      sha256 "$(sha_for "$darwin_arm")"
    end
    on_intel do
      url "${base}/${darwin_amd}"
      sha256 "$(sha_for "$darwin_amd")"
    end
  end

  on_linux do
    on_arm do
      url "${base}/${linux_arm}"
      sha256 "$(sha_for "$linux_arm")"
    end
    on_intel do
      url "${base}/${linux_amd}"
      sha256 "$(sha_for "$linux_amd")"
    end
  end

  def install
    bin.install "wiki"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/wiki version")
  end
end
EOF
)

if [[ -z "${HOMEBREW_TAP_TOKEN:-}" ]]; then
    echo "# DRY RUN (HOMEBREW_TAP_TOKEN unset) — formula below, not pushed:" >&2
    printf '%s\n' "$formula"
    exit 0
fi

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT
git clone --depth 1 "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/${TAP_REPO}.git" "$workdir/tap"

mkdir -p "$workdir/tap/Formula"
printf '%s\n' "$formula" >"$workdir/tap/Formula/${FORMULA}.rb"

git -C "$workdir/tap" config user.name "github-actions[bot]"
git -C "$workdir/tap" config user.email "github-actions[bot]@users.noreply.github.com"
git -C "$workdir/tap" add "Formula/${FORMULA}.rb"
if git -C "$workdir/tap" diff --cached --quiet; then
    echo "Formula already up to date for ${VERSION}; nothing to push."
    exit 0
fi
git -C "$workdir/tap" commit -m "${FORMULA} ${version}"
git -C "$workdir/tap" push
echo "Pushed ${FORMULA} ${version} to ${TAP_REPO}"
