cask "thawts" do
  version "0.3.0"

  on_arm do
    sha256 "REPLACE_WITH_ARM64_SHA256"
    url "https://github.com/OWNER/thawts-client-go/releases/download/v#{version}/thawts_darwin_arm64.zip"
  end

  on_intel do
    sha256 "REPLACE_WITH_AMD64_SHA256"
    url "https://github.com/OWNER/thawts-client-go/releases/download/v#{version}/thawts_darwin_amd64.zip"
  end

  name "Thawts"
  desc "Thought capture and review"
  homepage "https://thawts.app"

  app "thawts"

  zap trash: [
    "~/.thawts",
  ]
end
