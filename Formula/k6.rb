class K6 < Formula
	desc "A modern load testing tool for developers and testers"
	homepage "https://k6.io/"
	url "https://github.com/cheuk0324/k6/archive/refs/tags/v0.39.0.tar.gz"
	sha256 "46ecbd2bbf20634664e319b0c15526d580852c2e95b21900b0d2263b4bc44f8b"
	license "AGPL-3.0-only"
	depends_on "go" => :build
	def install
	  system "make"
	  bin.install "k6"
	end
	test do
	  system "#{bin}/k6", "--version"
	end
  end
