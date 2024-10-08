#! /usr/bin/env nix-shell
#! nix-shell -i bash -p fpm

set -euxo pipefail

PY_VER=$(head -n 1 robot_failover.pex  | grep -o "python3.*")
PKG_VER=$(git describe --tags | sed "s|v||g")

rm -fv *.deb
fpm -s dir -v "$PKG_VER" -d "$PY_VER" -t deb -n robot-failover --prefix /usr/bin robot_failover.pex=robot_failover
mv *.deb robot_failover.deb
