{ config, pkgs, lib, ... }:

with lib;

let
  cfg = config.services.robot-failover;
in
{
  options = {
    services.robot-failover = with types; {
      enable = mkEnableOption "mgit hetzner robot failover";

      thisRouterID = mkOption {
        description = "Virtual router ID of this machine's failover IPs";
        type = types.int;
      };

      common = {
        urlFloating = mkOption {
          description = "Endpoint for failover switching";
          internal = true;
          default = "https://robot-ws.your-server.de/failover/{}";
          type = types.str;
        };

        ipv6Suffix = mkOption {
          description = "IPv6 Suffix";
          default = "2";
          type = types.str;
        };

        floatingIPs = mkOption {
          description = "Floating IPs";
          type = types.listOf (types.submodule ({
            options = {
              ip = mkOption {
                type = types.str;
                description = "Floating IP without netmask and ipv6 suffix";
              };

              router = mkOption {
                type = types.int;
                description = "Virtual router id";
              };
            };
          }));
        };

        interface = mkOption {
          description = "Interface where to assign IPs";
          type = types.str;
          example = "enpXYZ";
        };

        robotUser = mkOption {
          description = "Robot user";
          type = types.nullOr types.str;
          example = "K123";
          default = null;
        };

        robotPassword = mkOption {
          description = "Robot password for all servers";
          type = types.nullOr types.str;
          default = null;
        };

        robotPasswords = mkOption {
          description = "Robot password for individual servers";
          type = types.nullOr (types.attrsOf types.str);
          example = {
            "1" = "...";
          };
          default = null;
        };
      };
    };
  };

  config = mkIf (cfg.enable) {
    environment.systemPackages = with pkgs; [
      robot-failover
    ];

    environment.etc."robot-failover/config.json".text = builtins.toJSON {
      this_router_id = cfg.thisRouterID;
      iproute2_bin = "${pkgs.iproute2}/bin";
      url_floating = cfg.common.urlFloating;
      ipv6_suffix = cfg.common.ipv6Suffix;
      floating_ips = cfg.common.floatingIPs;
      interface = cfg.common.interface;
      robot_user = cfg.common.robotUser;
      # assert one of these two if not use_vlan_ips
      robot_password = cfg.common.robotPassword;
      robot_passwords = cfg.common.robotPasswords;
    };

    services.keepalived = {
      enable = true;

      vrrpInstances = let
        uniqueRouters = unique (map (i: i.router) cfg.common.floatingIPs);
      in listToAttrs (map (router: nameValuePair ("robot_${toString router}") {
        interface = cfg.common.interface;
        state = "BACKUP";
        priority = if cfg.thisRouterID != router then router else cfg.thisRouterID + 10;
        virtualRouterId = router;
      }) uniqueRouters);
    };
  };
}
