# nix/modules/home-manager.nix — auto-generated typed module
# description: pleme-io's CLI framework for Go — the counterpart to the Rust clap / caixa-clap model: named app, subcommand tree, per-flag validators, multi-auth resolver.
{ config, lib, pkgs, ... }: let
  cfg = config.programs.cli-go;
in
{
  config = lib.mkIf cfg.enable {
    home.packages = [
      cfg.package
    ];
  };
  options.programs.cli-go = {
    enable = lib.mkEnableOption "cli-go";
    package = lib.mkOption {
      default = pkgs.cli-go or null;
      type = lib.types.package;
    };
  };
}
