{
  description = "meta-cli — CLI and MCP server for the Meta Marketing API";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
      in
      {
        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gopls
            gotools
            go-tools
            golangci-lint
            goreleaser
            lefthook
            just
          ];

          shellHook = ''
            echo "meta-cli development environment"
            echo "Go version: $(go version)"
            echo ""
            echo "Available commands:"
            echo "  just build  - Build the meta binary"
            echo "  just test   - Run tests"
            echo "  just lint   - Run linter"
            echo "  just install - Install to GOBIN"
          '';
        };
      }
    );
}
