{
  buildGoModule,
  lib,
}:

buildGoModule rec {
  pname = "failover-daemon";
  version = "0.0.0";

  src = ./.;

  vendorHash = null;

  meta = with lib; {
    description = "Mock Hetzner Failover Service Daemon";
    homepage = "https://github.com/mgit-at/hetzner-robot-failover-keepalived.git";
    license = licenses.mit;
    maintainers = with maintainers; [ mkg20001 ];
  };
}
