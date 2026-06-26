#!/bin/bash
# tsc typecheck with timeout
cd /usr/local/Wywy-Website/Wywy-CI/astro
node ./node_modules/.bin/tsc --noEmit > /tmp/tsc-out.txt 2>&1
STATUS=$?
echo "tsc exit=$STATUS" >> /tmp/tsc-out.txt
exit $STATUS
