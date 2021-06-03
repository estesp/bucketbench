#!/bin/bash

set -eu -o pipefail

go mod tidy
if [ -d vendor ]; then
   rm -rf vendor/
   go mod vendor
fi

DIFF_PATH="vendor/ go.mod go.sum"

# need word splitting here to avoid reading the whole DIFF_PATH as one pathspec
#
# shellcheck disable=SC2046
DIFF=$(git status --porcelain -- $DIFF_PATH)

if [ "$DIFF" ]; then
    echo
    echo "These files were modified:"
    echo
    echo "$DIFF"
    echo
    exit 1
else
    echo "$DIFF_PATH is correct"
fi
