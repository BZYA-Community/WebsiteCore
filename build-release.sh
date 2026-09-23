#!/bin/sh
# eg.1 : sh build-release.sh
# eg.2, set tags: sh build-release.sh 'go_json'

TAGS='go_json'

if [ -n "$1" ]; then
  TAGS="$1"
fi

make release CGO_ENABLED=0 TAGS="$TAGS"
