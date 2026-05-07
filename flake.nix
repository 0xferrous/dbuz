{
  description = "dbuz D-Bus TUI";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "dbuz";
          version = "0.1.0";
          src = ./.;

          vendorHash = "sha256-3vEtia3sTnsHvDaRskgfFl6X1OwDg0kJSDOYBA3ksYY=";

          meta = {
            description = "Terminal UI for exploring and interacting with D-Bus";
            mainProgram = "dbuz";
          };
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            gotools
            go-tools
          ];
        };
      });
}
