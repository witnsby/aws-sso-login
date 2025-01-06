class AwsSsoLogin < Formula
  desc "CLI that streamlines AWS SSO authentication and credentials management"
  homepage "https://github.com/witnsby/aws-sso-login"
  url "https://github.com/witnsby/aws-sso-login/archive/refs/tags/v0.0.5.tar.gz"
  sha256 "4f2b621e2eaf5649dd5ea87cb2516868a64e1fe854d92ed7c8dd4b02f924be6f"
  license "Apache-2.0"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(output: bin/"aws-sso-login"), "./cmd/aws-sso-login"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/aws-sso-login version")
  end
end
