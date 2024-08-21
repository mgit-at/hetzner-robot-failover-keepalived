{ config, pkgs, lib, ... }:

with lib;

let
  cfg = config.services.failover-daemon;
  type = pkgs.formats.json {};
in
{
  options.services.failover-daemon = {
    enable = mkEnableOption "hetzner mock failover daemon";
    config = mkOption {
      type = type.type;
      description = "Config for mock failover daemon";
    };
  };

  config = mkIf (cfg.enable) {
    systemd.services.failover-daemon = {
      path = with pkgs; [ failover-daemon iproute2 ];
      script = ''
        failover-daemon ${type.generate "config.json" cfg.config}
      '';
    };
  };
}
