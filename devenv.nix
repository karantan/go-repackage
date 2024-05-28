{ pkgs, ... }:

{
  # https://devenv.sh/packages/
  packages = [ pkgs.nodePackages.serverless ];
}
