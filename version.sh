#!/bin/sh

DESCRIBE=$(git describe --tags 2>/dev/null || echo 0.1.0)
if [ -z "$(git status --porcelain)" ]; then
    echo $DESCRIBE
else
    echo $DESCRIBE-dirty
fi