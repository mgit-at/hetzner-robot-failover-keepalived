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

              owner = mkOption {
                type = types.nullOr types.str;
                description = "Router id of server to which this IP belongs";
                default = null;
              };
            };
          }));
        };

        mainIPs = mkOption {
          description = "main IPs";
          type = types.attrsOf (types.submodule ({
            options = {
              ipv4 = mkOption {
                type = types.str;
                description = "Main IPv4 without netmask";
              };

              ipv6 = mkOption {
                type = types.str;
                description = "Main IPv6 without netmask and suffix";
              };
            };
          }));
        };

        interface = mkOption {
          description = "Interface where to assign IPs";
          type = types.str;
          example = "enpXYZ";
        };

        keepaliveInterface = mkOption {
          description = "Interface where to broadcast keepalive messages";
          type = types.str;
          example = "enpXYZ";
        };

        robotAuth = mkOption {
          description = "Robot user:pass for all servers";
          type = types.nullOr types.str;
          default = null;
        };

        robotAuths = mkOption {
          description = "Robot user:pass for individual servers";
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
      iproute2_bin = "${pkgs.iproute2}/bin/ip";
      url_floating = cfg.common.urlFloating;
      ipv6_suffix = cfg.common.ipv6Suffix;
      floating_ips = cfg.common.floatingIPs;
      main_ips = cfg.common.mainIPs;
      interface = cfg.common.interface;
      # assert one of these two if not use_vlan_ips
      robot_auth = cfg.common.robotAuth;
      robot_auths = cfg.common.robotAuths;
    };

    services.keepalived = {
      enable = true;

      vrrpInstances = let
        uniqueRouters = unique (map (i: i.router) cfg.common.floatingIPs);
      in listToAttrs (map (router: nameValuePair ("robot_${toString router}") {
        interface = cfg.common.keepaliveInterface;
        state = if cfg.thisRouterID != router then "BACKUP" else "MASTER";
        priority = if cfg.thisRouterID != router then router else cfg.thisRouterID + 10;
        virtualRouterId = router;
        extraConfig = ''
          notify "${pkgs.robot-failover}/bin/robot_failover ${toString router}"
        '';
      }) uniqueRouters);
    };
  };
}
