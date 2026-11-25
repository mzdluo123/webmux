#!/bin/sh
# Run from the repository root. Output is local build input, never installed.
set -eu

go=$1
shift
pins=internal/frphelper/pins.txt
base=bin/.frpc

fail() {
	printf 'prepare-frpc: %s\n' "$*" >&2
	exit 1
}

digest() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

matches() {
	case $2 in
	'' | *[!0-9a-f]*) return 1 ;;
	esac
	[ "${#2}" -eq 64 ] || return 1
	[ -f "$1" ] && [ "$(digest "$1")" = "$2" ]
}

version=$(awk '$1 == "version" { print $2 }' "$pins")
revision=$(awk '$1 == "version" { print $3 }' "$pins")
license_pin=$(awk '$1 == "license" { print $2 }' "$pins")
mkdir -p "$base"
stage=$(mktemp -d "$base/stage.XXXXXXXX")
trap 'rm -rf "$stage"' EXIT
trap 'exit 1' HUP INT TERM

for platform do
	os=${platform%-*}
	arch=${platform#*-}
	case $platform in
	linux-amd64 | linux-arm64 | darwin-amd64 | darwin-arm64) ;;
	*) fail "unsupported platform $platform" ;;
	esac
	archive_pin=$(awk -v os="$os" -v arch="$arch" '$1 == os && $2 == arch { print $3 }' "$pins")
	binary_pin=$(awk -v os="$os" -v arch="$arch" '$1 == os && $2 == arch { print $4 }' "$pins")
	dir=$base/$platform
	mkdir -p "$dir"
	if ! matches "$dir/frpc" "$binary_pin" || ! matches "$dir/LICENSE" "$license_pin"; then
		name=frp_${version}_${os}_${arch}
		archive=$stage/$name.tar.gz
		curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' \
			--connect-timeout 15 --max-time 180 \
			"https://github.com/fatedier/frp/releases/download/v$version/$name.tar.gz" --output "$archive"
		matches "$archive" "$archive_pin" || fail "$platform archive SHA-256 mismatch"
		tar -xOzf "$archive" "$name/frpc" > "$stage/frpc"
		tar -xOzf "$archive" "$name/LICENSE" > "$stage/LICENSE"
		matches "$stage/frpc" "$binary_pin" || fail "$platform frpc SHA-256 mismatch"
		matches "$stage/LICENSE" "$license_pin" || fail "$platform license SHA-256 mismatch"
		mv -f "$stage/frpc" "$dir/frpc"
		mv -f "$stage/LICENSE" "$dir/LICENSE"
		rm "$archive"
	fi

	# Read the Go build metadata without executing a foreign target binary.
	if ! "$go" version -m "$dir/frpc" | awk -v os="$os" -v arch="$arch" -v version="v$version" -v revision="$revision" '
		$1 == "path" && $2 == "github.com/fatedier/frp/cmd/frpc" { command=1 }
		$1 == "mod" && $2 == "github.com/fatedier/frp" && $3 == version { module=1 }
		$1 == "build" && $2 == "GOOS=" os { goos=1 }
		$1 == "build" && $2 == "GOARCH=" arch { goarch=1 }
		$1 == "build" && $2 == "vcs.revision=" revision { commit=1 }
		$1 == "build" && $2 == "vcs.modified=false" { clean=1 }
		END { exit !(command && module && goos && goarch && commit && clean) }
	'; then
		fail "$platform frpc build provenance mismatch"
	fi

	printf '{"Replace":{"internal/frphelper/payload/frpc":"%s/frpc","internal/frphelper/payload/LICENSE":"%s/LICENSE"}}\n' \
		"$dir" "$dir" > "$stage/overlay.json"
	mv -f "$stage/overlay.json" "$dir/overlay.json"
done
