{
  description = "dbus-debug Go TUI";

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
          pname = "dbus-debug";
          version = "0.1.0";
          src = ./.;

          vendorHash = "sha256-H/yok1zYhLhNUgykEGSmcv4zdqQRhn6UXU8sQhjakpE=";

          meta = {
            description = "Go TUI for D-Bus debugging";
            mainProgram = "dbus-debug";
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
