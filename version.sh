#!/bin/sh

if [ -z "$(git status --porcelain)" ]; then
    echo ${CIRCLE_TAG}
else
    echo ${CIRCLE_TAG}-dirty
fi