{ pkgs, ... }:

{
  # https://devenv.sh/packages/
  packages = [ pkgs.nodePackages.serverless ];
  languages.go.enable = true;
}
