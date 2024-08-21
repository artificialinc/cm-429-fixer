#!/bin/sh

if [ $# -eq 3 ]; then
  docker build --build-arg builduser=$1 --build-arg buildtoken=$2 -t ghcr.io/artificialinc/cm-429-fixer:$3 . --push --platform linux/amd64,linux/arm64
else
  echo "USAGE:  build.sh <GitHub Username> <GitHub Personal Access Token> <Image:Tag>"
fi
