#! /usr/bin/env nix-shell
#! nix-shell -i bash -p poetry -p python311

set -euxo pipefail

poetry install
poetry run pip freeze > requirements.txt
poetry run pex . -o robot_failover.pex -e robot_failover:cli -r requirements.txt --python python3.11
