#!/bin/bash

set -euxo pipefail

poetry run pex . -o failover.pex -e .:robot_failover:main --sources-directory=.
