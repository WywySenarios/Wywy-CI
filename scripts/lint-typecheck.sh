#!/bin/bash
cd /usr/local/Wywy-Website/Wywy-CI/astro && npx tsc --noEmit > /tmp/tsc-out.txt 2>&1
echo "tsc exit=$?" >> /tmp/tsc-out.txt
cd /usr/local/Wywy-Website/Wywy-CI && go vet ./server/api/ ./server/store/ > /tmp/govet-out.txt 2>&1
echo "vet exit=$?" >> /tmp/govet-out.txt
