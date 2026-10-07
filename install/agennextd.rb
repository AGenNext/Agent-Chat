# Homebrew formula for agennextd (Autonomyx / AGenNext Chat daemon).
#
# Install via a tap:
#   brew tap AGenNextHub/tap https://github.com/AGenNextHub/Agent-Chat
#   brew install agennextd
#
# Maintainers: on each release, run `scripts/build-macos.sh`, upload the
# tarballs to the GitHub release, and paste the sha256 values from
# dist/checksums-darwin.txt below (replacing the REPLACE_WITH_* placeholders).
class Agennextd < Formula
  desc "Headless agentic chat daemon — Autonomyx / AGenNext Chat"
  homepage "https://platform.openautonomyx.com"
  version "0.1.0"
  # TODO: confirm licence (open-source decision pending). Apache-2.0 assumed.
  license "Apache-2.0"

  on_macos do
    on_arm do
      url "https://github.com/AGenNextHub/Agent-Chat/releases/download/v0.1.0/agennextd_0.1.0_darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_ARM64_SHA256"
    end
    on_intel do
      url "https://github.com/AGenNextHub/Agent-Chat/releases/download/v0.1.0/agennextd_0.1.0_darwin_amd64.tar.gz"
      sha256 "REPLACE_WITH_AMD64_SHA256"
    end
  end

  def install
    bin.install "agennextd"
  end

  test do
    # -demo runs one turn end-to-end and exits 0.
    system "#{bin}/agennextd", "-demo"
  end
end
