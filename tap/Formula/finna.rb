# typed: false
# frozen_string_literal: true

# This file is a TEMPLATE showing what goreleaser will push to the
# acarmisc/homebrew-finna tap when a release tag is created.
# The {{VERSION}}, {{SHA256_*}}, and {{URL_*}} placeholders are filled by
# goreleaser automatically from the release artifacts.
#
# You do NOT need to edit this file manually — it is generated on every
# `git tag v* && git push --tags` by the .github/workflows/release.yml workflow.
#
# To set up the tap repo:
#   1. Create https://github.com/acarmisc/homebrew-finna (public)
#   2. Create a fine-grained PAT with "Contents: write" on that repo
#   3. Store it as HOMEBREW_TAP_TOKEN in the finna-cli repo's Actions secrets

class Finna < Formula
  desc "CLI for finna-app cloud cost management"
  homepage "https://github.com/acarmisc/finna-cli"
  version "{{VERSION}}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/acarmisc/finna-cli/releases/download/v{{VERSION}}/finna_{{VERSION}}_darwin_arm64.tar.gz"
      sha256 "{{SHA256_DARWIN_ARM64}}"
    else
      url "https://github.com/acarmisc/finna-cli/releases/download/v{{VERSION}}/finna_{{VERSION}}_darwin_amd64.tar.gz"
      sha256 "{{SHA256_DARWIN_AMD64}}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/acarmisc/finna-cli/releases/download/v{{VERSION}}/finna_{{VERSION}}_linux_arm64.tar.gz"
      sha256 "{{SHA256_LINUX_ARM64}}"
    else
      url "https://github.com/acarmisc/finna-cli/releases/download/v{{VERSION}}/finna_{{VERSION}}_linux_amd64.tar.gz"
      sha256 "{{SHA256_LINUX_AMD64}}"
    end
  end

  def install
    bin.install "finna"
    generate_completions_from_executable(bin/"finna", "completion")
  end

  test do
    system "#{bin}/finna", "version"
  end
end
