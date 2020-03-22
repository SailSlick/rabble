#!/usr/bin/env sh

set -e

cd /repo

cd chump/ && npm run lint && npm run test && cd ..

echo "Running go tests"
go test github.com/cpssd/rabble/...

echo "Running python unit tests for activites/undo"
cd build_out
python3 -B -m unittest discover
