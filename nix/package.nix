{self, ...}: {
  perSystem = {
    inputs',
    lib,
    pkgs,
    self',
    ...
  }: {
    packages = {
      gast = (pkgs.buildGoModule.override {go = inputs'.devtools.packages.go;}) {
        pname = "gast";
        # Nix has no version to read from the source tree, so builds from a
        # checkout are stamped with the commit date.
        version = "0-unstable-${self.lastModifiedDate or "0"}";

        src = lib.fileset.toSource {
          root = ../.;
          fileset = lib.fileset.difference (lib.fileset.gitTracked ../.) (
            lib.fileset.unions [
              ../flake.nix
              ../flake.lock
              ../nix
            ]
          );
        };

        # Empty while the module has no dependencies. Once go.mod grows one, run
        # `nix build` and put the reported got: hash here.
        vendorHash = null;

        env.CGO_ENABLED = 0;

        # The template test suite is pure Go and safe to run in the sandbox.
        doCheck = true;

        meta = {
          description = "Starting point for new Go projects";
          homepage = "https://github.com/stubbedev/gast";
          license = lib.licenses.mit;
          mainProgram = "gast";
          platforms = lib.platforms.unix;
        };
      };
      default = self'.packages.gast;
    };

    apps = {
      gast = {
        type = "app";
        program = lib.getExe self'.packages.gast;
      };
      default = self'.apps.gast;
    };
  };
}
