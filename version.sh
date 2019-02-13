#!/bin/sh

if [ -z "$(git status --porcelain)" ]; then
    echo ${CIRCLE_TAG}
else
    git status 1>&2
    echo ${CIRCLE_TAG}-dirty
fi