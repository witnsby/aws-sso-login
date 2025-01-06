class AwsSsoLogin < Formula
  desc "CLI that streamlines AWS SSO authentication and credentials management"
  homepage "https://github.com/witnsby/aws-sso-login"
  url "https://github.com/witnsby/aws-sso-login/archive/refs/tags/v0.0.5.tar.gz"
  sha256 "b0f95257ffd312de26dddeba59eb76b9e595e862418e5b9b0ec2707090e34e37"
  license "Apache-2.0"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(output: bin/"aws-sso-login"), "./cmd/aws-sso-login"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/aws-sso-login version")
  end
end
