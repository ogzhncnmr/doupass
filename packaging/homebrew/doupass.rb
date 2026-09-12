class Doupass < Formula
  desc "Local-first policy engine for AI coding agents"
  homepage "https://github.com/ogzhncnmr/doupass"
  version "0.1.0-rc4"
  license "Apache-2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/ogzhncnmr/doupass/releases/download/v0.1.0-rc4/doupass_0.1.0-rc4_darwin_arm64.tar.gz"
      sha256 "2d9a0afecb641509093de245bb1f231f8d01a40f7deae0dca1844e44fd446c34"
    else
      url "https://github.com/ogzhncnmr/doupass/releases/download/v0.1.0-rc4/doupass_0.1.0-rc4_darwin_amd64.tar.gz"
      sha256 "046fa0c5bf43626579490f9589fb7439d9447f8576aebe28aad921dcd745cab9"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/ogzhncnmr/doupass/releases/download/v0.1.0-rc4/doupass_0.1.0-rc4_linux_arm64.tar.gz"
      sha256 "32b932c102326c519817788cc58e48b35b413f3b58396251c9ec780e31e4acde"
    else
      url "https://github.com/ogzhncnmr/doupass/releases/download/v0.1.0-rc4/doupass_0.1.0-rc4_linux_amd64.tar.gz"
      sha256 "67632e1cf261015ead68382df770afe9fa09b2f07bd6e7609303cd6eae4d6ade"
    end
  end

  def install
    bin.install "doupass"
  end

  test do
    assert_match "doupass", shell_output("#{bin}/doupass version")
  end
end
