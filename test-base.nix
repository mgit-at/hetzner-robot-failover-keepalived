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
      floatingIPs = [
        { ip = "10.42.0.1"; router = 1; }
        { ip = "42::1"; router = 1; }
        { ip = "10.42.0.2"; router = 2; }
        { ip = "42::2"; router = 2; }
      ];
    };
  };

  networking = {
    nftables.enable = true;
    firewall.allowedTCPPorts = [ 80 ];
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
      tcpdump -lni any '(net 10.42.0.0/16 or net 42::/16) and tcp'
    '';
    wantedBy = [ "multi-user.target" ];
    after = [ "network.target" ];
  };
}
