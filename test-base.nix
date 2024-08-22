{ config, pkgs, ... }: {
  imports = [
    ./module.nix
  ];

  virtualisation.vlans = [ 1 ];
  networking.vlans.hetzner = {
    id = 1;
    interface = "eth1";
  };

  services.robot-failover = {
    enable = true;
    common = {
      interface = "hetzner";
      keepaliveInterface = "hetzner";
      floatingIPs = [
        { ip = "42.0.0.1"; router = 1; }
        { ip = "42:1::"; router = 1; } # will be 42:1::2
        { ip = "42.0.0.2"; router = 2; }
        { ip = "42:2::"; router = 2; } # will be 42:2::2
      ];
      mainIPs = {
        "1" = {
          ipv4 = "10.42.0.1";
          ipv6 = "fe42:1::"; # will be fe42:1::2
        };
        "2" = {
          ipv4 = "10.42.0.2";
          ipv6 = "fe42:2::"; # will be fe42:2::2
        };
      };
      urlFloating = "http://10.42.0.254:9090/{0}";
      robotAuths = {
        "1" = "1:1234";
        "2" = "2:1234";
      };
    };
  };

  systemd.services.keepalived.environment.FORCE_DEBUG_FAILOVER = "1";
  systemd.timers.keepalived-boot-delay.enable = false;

  networking = {
    nftables.enable = true;
    firewall.allowedTCPPorts = [ 80 ];
    firewall.extraInputRules = ''
      ip protocol vrrp accept
    '';
  };

  networking.defaultGateway = {
    interface = "hetzner";
    address = "10.42.0.254";
  };
  networking.defaultGateway6 = {
    interface = "hetzner";
    address = "fe42::254";
  };

  services.nginx = {
    enable = true;
    virtualHosts.default = {
      default = true;
      locations."/".return = "200 server-${config.networking.hostName}";
    };
  };
}
