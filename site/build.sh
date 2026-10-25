#!/bin/sh
# Copies the demo recordings and logo the page uses into site/media, from
# the repo, so the site never carries its own copies. Vercel runs this as
# the build; run it once locally to preview.
set -e
cd "$(dirname "$0")"
mkdir -p media
for gif in full-tour devices navigation nowplaying lyrics queue cli; do
  cp "../demo/recordings/$gif.gif" media/
done
cp ../assets/logo.png media/
