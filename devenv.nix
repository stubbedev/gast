{
  pkgs,
  ...
}: {
  packages = with pkgs; [
    go_1_27
    gopls
    golangci-lint
    delve
    just
    git
    gh
  ];

  env.CGO_ENABLED = "0";
  env.GOTOOLCHAIN = "local";
}
