{
  lib,
  buildGo127Module,
  # Set by the flake from `self.shortRev or self.dirtyShortRev`; falls back to
  # the placeholder used when nothing was stamped in.
  rev ? "unknown",
  version ? "0-unstable",
}:
buildGo127Module {
  pname = "gast";
  inherit version;

  src = lib.cleanSource ./.;

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
}
