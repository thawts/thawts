#!/bin/bash

# get directory contexts
CURRENTDIR=$(pwd)
PROJECTDIR=$(git rev-parse --show-toplevel)

# go to root of git repository and return to original directory
# on exit
cd $PROJECTDIR
trap 'cd "$CURRENTDIR"' EXIT

# run tests
go test ./... || exit 1

# build application
wails build || exit 1

# ask if app should be started - if user does not answer within
# 5 seconds end script without running app
read -t 5 -p "Start application? [y/N] "
if [ $? -eq 0 ]; then
    open ./build/bin/thawts.app
fi
