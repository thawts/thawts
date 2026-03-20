# This file is updated automatically by GoReleaser on each release.
# Manual edits to version/sha256 will be overwritten on next release.
class Thawts < Formula
  desc "Thought capture and review"
  homepage "https://thawts.app"
  version "0.3.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/OWNER/thawts-client-go/releases/download/v#{version}/thawts_darwin_arm64.zip"
      sha256 "REPLACE_WITH_ARM64_SHA256"
    end

    on_intel do
      url "https://github.com/OWNER/thawts-client-go/releases/download/v#{version}/thawts_darwin_amd64.zip"
      sha256 "REPLACE_WITH_AMD64_SHA256"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/OWNER/thawts-client-go/releases/download/v#{version}/thawts_linux_arm64.zip"
      sha256 "REPLACE_WITH_LINUX_ARM64_SHA256"
    end

    on_intel do
      url "https://github.com/OWNER/thawts-client-go/releases/download/v#{version}/thawts_linux_amd64.zip"
      sha256 "REPLACE_WITH_LINUX_AMD64_SHA256"
    end
  end

  def install
    bin.install "thawts"
  end

  test do
    system "#{bin}/thawts", "--version"
  end
end
