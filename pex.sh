#!/bin/bash

set -euxo pipefail

poetry install
poetry run pip freeze > requirements.txt
poetry run pex . -o failover.pex -e robot_failover:main -r requirements.txt

