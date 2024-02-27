{
  description = "Hetzner Robot - Failover IP and Private IP switchover - keepalived";

  inputs.nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";

  outputs = { self, nixpkgs }@inputs: let
    inherit (self) outputs;

    supportedSystems = [ "x86_64-linux" ];
    forAllSystems = f: nixpkgs.lib.genAttrs supportedSystems (system: f system);

    gen = system: (let
      pkgs = import "${nixpkgs}" {
        system = "x86_64-linux";
        overlays = builtins.attrValues outputs.overlays;
      };
    in {
      packages.${system} = {
        default = pkgs.robot-failover;
        inherit (pkgs) robot-failover;
      };

      checks.${system} = {
        hcloud = pkgs.testers.runNixOSTest ./test.nix;
      };
    });
  in {
    overlays.default = import ./overlay.nix;

    nixosModules.hcloud = import ./module.nix;
  } // (gen "x86_64-linux");
}
