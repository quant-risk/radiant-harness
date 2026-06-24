# Homebrew formula for radiant-harness.
#
# Install locally for testing:
#   brew install --build-from-source ./packaging/homebrew/radiant.rb
#
# Once accepted into a tap (e.g. Fortvna/homebrew-tap):
#   brew install fortvna/tap/radiant
#
# Update VERSION and SHA256 below when releasing a new binary. Both
# values come from the GitHub release page (goreleaser publishes them
# as `radiant_<version>_<os>_<arch>.tar.gz` next to `checksums.txt`).

class Radiant < Formula
  desc "Spec-Driven Development harness for AI coding agents — Go CLI with auto-correction loop and multi-LLM routing"
  homepage "https://github.com/Fortvna-Risk-Solutions/radiant-harness"
  url "https://github.com/Fortvna-Risk-Solutions/radiant-harness/releases/download/v0.2.2/radiant_0.2.2_darwin_arm64.tar.gz"
  sha256 "REPLACE_WITH_DARWIN_ARM64_SHA256"
  license "MIT"

  # Multi-architecture support. Homebrew selects the right URL automatically
  # based on the user's Mac (Intel vs Apple Silicon).
  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/Fortvna-Risk-Solutions/radiant-harness/releases/download/v0.2.2/radiant_0.2.2_darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_DARWIN_ARM64_SHA256"
    else
      url "https://github.com/Fortvna-Risk-Solutions/radiant-harness/releases/download/v0.2.2/radiant_0.2.2_darwin_amd64.tar.gz"
      sha256 "REPLACE_WITH_DARWIN_AMD64_SHA256"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/Fortvna-Risk-Solutions/radiant-harness/releases/download/v0.2.2/radiant_0.2.2_linux_arm64.tar.gz"
      sha256 "REPLACE_WITH_LINUX_ARM64_SHA256"
    else
      url "https://github.com/Fortvna-Risk-Solutions/radiant-harness/releases/download/v0.2.2/radiant_0.2.2_linux_amd64.tar.gz"
      sha256 "REPLACE_WITH_LINUX_AMD64_SHA256"
    end
  end

  # `radiant` is a single self-contained binary — no runtime dependencies,
  # no shell scripts, no app bundles to maintain.
  def install
    bin.install "radiant"
  end

  test do
    # Smoke test: --version must print the version we shipped.
    assert_match version.to_s, shell_output("#{bin}/radiant --version")
    # --help must list the documented commands.
    assert_match "init", shell_output("#{bin}/radiant --help")
    assert_match "validate", shell_output("#{bin}/radiant --help")
    assert_match "run", shell_output("#{bin}/radiant --help")
  end
end
