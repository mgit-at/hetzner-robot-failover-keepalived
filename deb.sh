#! /usr/bin/env nix-shell
#! nix-shell -i bash -p fpm

set -euxo pipefail

rm -fv *.deb
fpm -s dir -t deb -n robot-failover --prefix /usr/bin robot_failover.pex
mv *.deb robot_failover.deb
