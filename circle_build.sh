#!/bin/sh

mkdir -p ./bin

exec docker run -it -v "$PWD:/go/src/$IMPORT_PATH" -v "$PWD/bin:/go/bin" golang:1.6 go install  --ldflags '-extldflags "-static"' $IMPORT_PATH
