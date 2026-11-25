#!/bin/sh
# Verify the managed inputs in the exact platform files to be signed.
set -eu

dir=$1
version=$2
repo=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
case $version in
*-dev.*) prefix=webmux-$version- ;;
*) prefix=webmux- ;;
esac

listed() {
	awk -v name="$1" '$2 == name { count++ } END { exit count != 1 }' "$dir/SHA256SUMS"
}

listed "${prefix}frpc-pins.txt"
listed "${prefix}frpc-LICENSE.txt"
cmp "$repo/internal/frphelper/pins.txt" "$dir/${prefix}frpc-pins.txt"
for platform in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64; do
	name=$prefix$platform
	listed "$name"
	listed "$name.tar.gz"
	cmp "$dir/.frpc/$platform/LICENSE" "$dir/${prefix}frpc-LICENSE.txt"
	test "$(tar -tzf "$dir/$name.tar.gz")" = "$(printf 'webmux\nfrpc.LICENSE')"
	tar -xOzf "$dir/$name.tar.gz" webmux | cmp "$dir/$name" -
	tar -xOzf "$dir/$name.tar.gz" frpc.LICENSE | cmp "$dir/${prefix}frpc-LICENSE.txt" -
	python3 - "$dir/$name" "$dir/.frpc/$platform" <<'PY'
import mmap
from pathlib import Path
import sys

server, bundle = map(Path, sys.argv[1:])
with server.open('rb') as f, mmap.mmap(f.fileno(), 0, access=mmap.ACCESS_READ) as image:
    for asset in ('frpc', 'LICENSE'):
        if image.find((bundle / asset).read_bytes()) < 0:
            sys.exit(f'{server}: managed {asset} is missing or differs from the pinned input')
PY
done
