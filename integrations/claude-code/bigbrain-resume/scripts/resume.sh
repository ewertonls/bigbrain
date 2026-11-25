#!/bin/sh
set -eu
if [ "$#" -ne 3 ]; then
    echo 'usage: sh resume.sh DATABASE_DIR PROJECT SEARCH_TEXT' >&2
    exit 2
fi
case "$1" in
    /*) ;;
    *) echo 'database must be an absolute directory path' >&2; exit 2 ;;
esac
exec "${BIGBRAIN_BIN:-bigbrain}" --json --db "$1" query --project "$2" --text "$3" --mode associative
