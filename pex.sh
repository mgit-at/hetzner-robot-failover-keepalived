#! /usr/bin/env nix-shell
#! nix-shell -i bash -p poetry -p python311 -p python313

set -euxo pipefail

poetry install
poetry run pip freeze > requirements.txt
poetry run pex . -o robot_failover.pex -e robot_failover:cli -r requirements.txt --python python3.11 --python python3.13 --python-shebang "/usr/bin/env python3"
