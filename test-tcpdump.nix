{ config, pkgs, ... }: {
  systemd.services.tcpdump = {
    path = with pkgs; [ tcpdump ];
    script = ''
      tcpdump -lni any '(net 10.42.0.0/16 or net fe42::/64 or net 42.0.0.0/8 or net 42::/16) and tcp'
    '';
    wantedBy = [ "multi-user.target" ];
    after = [ "network.target" ];
  };
}
