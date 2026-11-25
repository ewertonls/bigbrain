#!/bin/sh
set -eu

test -z "$(command -v cc || true)"
test -z "$(command -v go || true)"

mkdir /tmp/install
tar -xzf /package.tar.gz -C /tmp/install
test -f /tmp/install/bigbrain/PACKAGE.md
export LD_LIBRARY_PATH=/tmp/install/bigbrain/lib

/tmp/install/bigbrain/bin/ladybug-spike schema --db /work/offline.lbdb
/tmp/install/bigbrain/bin/ladybug-spike seed --db /work/offline.lbdb
/tmp/install/bigbrain/bin/ladybug-spike verify --db /work/offline.lbdb
