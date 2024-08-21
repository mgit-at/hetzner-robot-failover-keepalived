{ pkgs, lib, ...} :
let
  shared = {
    imports = [
      ./test-base.nix
    ];
  };
in
{
  name = "mgit-robot-failover";

  # 42.0.0.0/8 failover IPs
  # 42::/8     failover IPs
  # 10.42.0.0/24 internal
  # fe42::/16    internal

  # client routes to daemon
  # daemon has via routes that forward traffic to router
  # routers have global routes to push traffic (back) out to daemon
  # daemon will route using 12::/8 route back to client

  nodes = {
    daemon = { lib, ... }: {
      imports = [
        ./failover-daemon/module.nix
        ./test-tcpdump.nix
      ];

      boot.kernel.sysctl = {
        "net.ipv4.conf.all.forwarding" = true;
        "net.ipv6.conf.all.forwarding" = true;
      };

      virtualisation.vlans = [ 1 2 ];
      networking.vlans.hetzner = {
        id = 1;
        interface = "eth1";
      };
      networking.vlans.client = {
        id = 2;
        interface = "eth1";
      };

      networking = {
        nftables.enable = true;
        firewall.allowedTCPPorts = [ 9090 ];
        firewall.filterForward = true;
        firewall.extraForwardRules = "accept";
      };

      networking.hostName = "daemon";
      networking.interfaces."hetzner".ipv4.addresses = [{
        address = "10.42.0.254";
        prefixLength = 16;
      }];
      networking.interfaces."hetzner".ipv6.addresses = [{
        address = "fe42::254";
        prefixLength = 16;
      }];
      networking.interfaces."client".ipv4.addresses = [{
        address = "12.0.0.1";
        prefixLength = 8;
      }];
      networking.interfaces."client".ipv6.addresses = [{
        address = "12::1";
        prefixLength = 16;
      }];

      services.failover-daemon = {
        enable = true;
        config = {
          servers = {
            "1" = {
              token = "1234";
              main = {
                v4 = "10.42.0.1";
                v6 = "fe42:1::";
              };
              failover = {
                v4 = "42.0.0.1";
                v6 = "42:1::";
              };
            };
            "2" = {
              token = "1234";
              main = {
                v4 = "10.42.0.2";
                v6 = "fe42:2::";
              };
              failover = {
                v4 = "42.0.0.2";
                v6 = "42:2::";
              };
            };
          };
          listen = "10.42.0.254:9090";
        };
      };
    };
    router1 = { lib, ... }: {
      imports = [
        shared
        ./test-tcpdump.nix
      ];

      networking.hostName = "router1";
      services.robot-failover.thisRouterID = 1;
      networking.interfaces."hetzner".ipv4.addresses = [{
        address = "10.42.0.1";
        prefixLength = 16;
      }];
      networking.interfaces."hetzner".ipv6.addresses = [{
        address = "fe42:1::2";
        prefixLength = 16;
      }];
    };
    router2 = { lib, ... }: {
      imports = [
        shared
        ./test-tcpdump.nix
      ];

      networking.hostName = "router2";
      services.robot-failover.thisRouterID = 2;
      networking.interfaces."hetzner".ipv4.addresses = [{
        address = "10.42.0.2";
        prefixLength = 16;
      }];
      networking.interfaces."hetzner".ipv6.addresses = [{
        address = "fe42:2::2";
        prefixLength = 16;
      }];
    };
    client = { lib, pkgs, ... }: {
      imports = [
        ./test-tcpdump.nix
      ];

      virtualisation.vlans = [ 1 2 ];
      networking.vlans.client = {
        id = 2;
        interface = "eth1";
      };
      networking.interfaces."client".ipv4.addresses = [{
        address = "12.0.0.2";
        prefixLength = 8;
      }];
      networking.interfaces."client".ipv4.routes = [{
        address = "42.0.0.0";
        prefixLength = 8;
        via = "12.0.0.1";
      }];
      networking.interfaces."client".ipv6.addresses = [{
        address = "12::2";
        prefixLength = 16;
      }];
      networking.interfaces."client".ipv6.routes = [{
        address = "42::";
        prefixLength = 16;
        via = "12::1";
      }];
      networking.defaultGateway = {
        interface = "client";
        address = "12.0.0.1";
      };
      networking.defaultGateway6 = {
        interface = "client";
        address = "12::1";
      };
      networking.hostName = "client";
      environment.systemPackages = with pkgs; [
        curl
        wget
      ];
    };
  };

  testScript = ''
    start_all()
    router1.wait_for_unit("nginx.service")
    router2.wait_for_unit("nginx.service")
    # router1.wait_for_unit("keepalived-boot-delay.timer")
    # router2.wait_for_unit("keepalived-boot-delay.timer")
    # router1.wait_for_unit("keepalived.service")
    # router2.wait_for_unit("keepalived.service")

    daemon.wait_for_unit("failover-daemon.service")

    client.succeed("sleep 44s")
    client.wait_for_unit("network.target")

    with subtest("daemon works"):
      router1.succeed("curl -v http://10.42.0.254:9090")

    with subtest("nginx running on local ips"):
      daemon.succeed("curl 10.42.0.1 | grep server-router1")
      daemon.succeed("curl [fe42:1::2] | grep server-router1")
      daemon.succeed("curl 10.42.0.2 | grep server-router2")
      daemon.succeed("curl [fe42:2::2] | grep server-router2")
      daemon.succeed("sleep 2s")

    # TODO: we can specify this in interface config
    with subtest("[workarround] replace default ipv6 router"):
      # qemu seems to advertise something via ra, let's replace it
      router1.succeed("ip -6 r r default via fe42::254 metric 50")
      router2.succeed("ip -6 r r default via fe42::254 metric 50")

    with subtest("router1 is serving 42.0.0.1 and 42:1::2"):
      router1.succeed("systemctl restart keepalived")
      router1.succeed("sleep 10s")
      daemon.succeed("ping 42.0.0.1 -w 1 -c 1")
      daemon.succeed("ping -6 42:1::2 -w 1 -c 1")
      client.succeed("curl 42.0.0.1 | grep server-router1")
      client.succeed("sleep 2s")
      client.succeed("curl [42:1::2] | grep server-router1")
      client.succeed("sleep 2s")

    with subtest("when router1 is offline router2 is serving 10.42.0.1 and 42:1::2"):
      router1.shutdown()
      router2.succeed("systemctl restart keepalived")
      router2.succeed("sleep 10s")
      daemon.succeed("ping 42.0.0.1 -w 1 -c 1")
      daemon.succeed("ping -6 42:1::2 -w 1 -c 1")
      client.succeed("curl -v 42.0.0.1 >&2")
      client.succeed("curl 42.0.0.1 | grep server-router2")
      client.succeed("sleep 2s")
      client.succeed("curl [42:1::2] | grep server-router2")
      client.succeed("sleep 2s")
  '';
}
