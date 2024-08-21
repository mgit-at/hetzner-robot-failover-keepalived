final: prev: {
  robot-failover = prev.callPackage ./. {};
  failover-daemon = prev.callPackage ./failover-daemon/package.nix {};
}
