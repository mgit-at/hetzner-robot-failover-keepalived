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
        { ip = "42::1"; router = 1; }
        { ip = "42.0.0.2"; router = 2; }
        { ip = "42::2"; router = 2; }
      ];
      mainIPs = {
        "1" = {
          ipv4 = "10.42.0.1";
          ipv6 = "fe42::1";
        };
        "2" = {
          ipv4 = "10.42.0.2";
          ipv6 = "fe42::2";
        };
      };
      urlFloating = "http://10.42.0.254/{ip}";
    };
  };

  networking = {
    nftables.enable = true;
    firewall.allowedTCPPorts = [ 80 ];
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

  systemd.services.tcpdump = {
    path = with pkgs; [ tcpdump ];
    script = ''
      tcpdump -lni any '(net 10.42.0.0/16 or net fe42::/64 or net 42.0.0.0/8 or net 42::/16) and tcp'
    '';
    wantedBy = [ "multi-user.target" ];
    after = [ "network.target" ];
  };
}
