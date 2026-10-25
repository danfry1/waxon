#!/bin/sh
# Builds the site into site/dist: the page and its files, plus the demo's
# album art, copied from demo/art so the site keeps no copy of its own.
# Vercel runs this as the build; run it locally to preview dist/.
set -e
cd "$(dirname "$0")"
rm -rf dist && mkdir dist
cp -R index.html demo.js mark.svg favicon.svg favicon.ico apple-touch-icon.png og.png video dist/
mkdir -p dist/media/art
cp ../demo/art/*.jpg dist/media/art/
