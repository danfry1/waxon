{
  description = "A vim-modal Spotify client for the terminal";

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
        pkgs = import nixpkgs { inherit system; };
        # Bumped on release (see CHANGELOG.md / the release checklist in
        # CLAUDE.md); CI checks it matches the newest CHANGELOG entry.
        version = "1.7.0";
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "waxon";
          inherit version;
          src = ./.;

          # Hash of the Go module dependencies. When go.mod/go.sum change,
          # set this to "" and copy the hash nix prints.
          vendorHash = "sha256-ub8VHazMiIFCJmeN6jdxYRaHwLgMzbE9YmFUD/KEnJY=";

          ldflags = [
            "-s"
            "-w"
            "-X main.version=v${version}"
          ];

          # The TUI's tests don't need a terminal, but the demo build tag
          # and VHS recordings are outside the package; plain tests are fine.
          doCheck = true;

          meta = {
            description = "A vim-modal Spotify client for the terminal";
            homepage = "https://github.com/danfry1/waxon";
            license = pkgs.lib.licenses.gpl3Only;
            mainProgram = "waxon";
          };
        };

        apps.default = flake-utils.lib.mkApp {
          drv = self.packages.${system}.default;
        };

        checks.default = self.packages.${system}.default;

        formatter = pkgs.nixfmt-rfc-style;

        devShells.default = pkgs.mkShell {
          packages = [
            pkgs.go
            pkgs.golangci-lint
            pkgs.vhs
          ];
        };
      }
    );
}
