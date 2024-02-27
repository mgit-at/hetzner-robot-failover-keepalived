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

  nodes = {
    router1 = { lib, ... }: {
      imports = [
        shared
      ];

      networking.hostName = "router1";
      services.robot-failover.thisRouterID = 1;
    };
    router2 = { lib, ... }: {
      imports = [
        shared
      ];

      networking.hostName = "router2";
      services.robot-failover.thisRouterID = 2;
    };
    client = { lib, pkgs, ... }: {
      networking.interfaces."eth1".ipv6.addresses = [{
        address = "42::";
        prefixLength = 16;
      }];
      networking.interfaces."eth1".ipv4.addresses = [{
        address = "10.42.0.0";
        prefixLength = 16;
      }];
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
    router1.wait_for_unit("keepalived.service")
    router2.wait_for_unit("keepalived.service")

    client.wait_for_unit("network.target")

    with subtest("router1 is serving 10.42.0.1 and 42::1"):
      client.succeed("curl 10.42.0.1 | grep server-router1")
      client.succeed("curl [42::1] | grep server-router1")

    with subtest("when router1 is offline router2 is serving 10.42.0.1 and 42::1"):
      client.succeed("curl 10.42.0.1 | grep server-router2")
      client.succeed("curl [42::1] | grep server-router2")
  '';
}
