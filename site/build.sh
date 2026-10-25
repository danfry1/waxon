#!/bin/sh
# Builds the site into site/dist: the page and its images, plus the demo
# recordings, copied from demo/ so the site keeps no copies of its own.
# Vercel runs this as the build; run it locally to preview dist/.
set -e
cd "$(dirname "$0")"
rm -rf dist
mkdir -p dist/media
cp index.html mark.svg favicon.svg og.png dist/
for gif in full-tour devices navigation nowplaying lyrics queue cli; do
  cp "../demo/recordings/$gif.gif" dist/media/
done
