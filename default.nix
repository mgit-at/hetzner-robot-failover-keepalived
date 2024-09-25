{
  lib,
  python3,
}:

python3.pkgs.buildPythonApplication rec {
  name = "hetzner-robot-failover-keepalived";
  pyproject = true;

  src = ./.;

  build-system = [
    python3.pkgs.poetry-core
    python3.pkgs.setuptools
    python3.pkgs.wheel
  ];

  dependencies = with python3.pkgs; [
    bunch
    requests
  ];

  postPatch = ''
    sed "s|os.path.dirname(__file__)|\"/etc/robot-failover\"|" -i robot_failover.py
  '';

  installPhase = ''
    install -D robot_failover.py "$out/bin/robot_failover"
  '';

  meta = with lib; {
    description = "Hetzner Robot - Failover IP and Private IP switchover with keepalived";
    homepage = "https://github.com/mgit-at/hetzner-robot-failover-keepalived";
    license = licenses.mit;
    maintainers = with maintainers; [ mkg20001 ];
    mainProgram = "robot_failover";
    platforms = platforms.all;
  };
}
