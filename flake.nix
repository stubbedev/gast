{
  description = "gast - starting point for new Go projects";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = nixpkgs.legacyPackages.${system};
        # Nix has no version to read from the source tree, so builds from a
        # checkout are stamped with the commit date and hash.
        rev = self.shortRev or self.dirtyShortRev or "unknown";
        version = "0-unstable-${self.lastModifiedDate or "0"}";
      in {
        packages = rec {
          gast = pkgs.callPackage ./package.nix {inherit rev version;};
          default = gast;
        };

        apps = rec {
          gast = flake-utils.lib.mkApp {drv = self.packages.${system}.gast;};
          default = gast;
        };

        formatter = pkgs.alejandra;
      }
    );
}
