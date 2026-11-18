#!/bin/sh
# Install WebMux from GitHub Releases on Linux or macOS.
#
# Usage:
#   curl -fsSL <install.sh URL> | sh
#   curl -fsSL <install.sh URL> | WEBMUX_VERSION=0.16.0 sh
#   curl -fsSL <install.sh URL> | WEBMUX_CHANNEL=dev sh
#
# Environment:
#   WEBMUX_CHANNEL      stable (default): the latest release; dev: the
#                       current development build from the rolling "dev"
#                       Release. Applies to this install only; nothing is saved.
#   WEBMUX_VERSION      X.Y.Z, or X.Y.Z-dev.N for the current development
#                       build (implies WEBMUX_CHANNEL=dev); a leading "v" is
#                       ignored (default: the latest of the channel). Releases
#                       older than v0.11.0 are unsigned and require WEBMUX_SHA256.
#   WEBMUX_INSTALL_DIR  Directory for the webmux binary (default: ~/.local/bin)
#   WEBMUX_MIRRORS      Space-separated gh-proxy prefixes; empty disables fallback
#   WEBMUX_SHA256       Pin the binary digest instead of reading SHA256SUMS
#   WEBMUX_BASE_URL     GitHub repository URL (mainly for tests and mirrors)
#
# GitHub is tried first, then the mirrors. Without WEBMUX_SHA256, SHA256SUMS is
# verified against SHA256SUMS.minisig and the built-in Ed25519 release public
# keys (via minisign or OpenSSL) before any binary digest from it is trusted.
# When neither minisign nor Ed25519-capable OpenSSL is installed, unsigned
# SHA256SUMS is accepted only directly from https://github.com/mzdluo123/webmux;
# mirrors are used only with WEBMUX_SHA256 or a digest already read from GitHub.
# The script never fetches from raw.githubusercontent.com; callers choose where
# to read this file.
#
# The "dev" Release keeps the current and the previous development build, but
# its SHA256SUMS lists only the current one: the dev channel installs the one
# webmux-<X.Y.Z-dev.N>-<os>-<arch> listed there (reading SHA256SUMS even with
# WEBMUX_SHA256), and a WEBMUX_VERSION other than that version fails.

set -eu

canonical_repo=https://github.com/mzdluo123/webmux
repo_base=${WEBMUX_BASE_URL:-$canonical_repo}
repo_base=${repo_base%/}
mirrors=${WEBMUX_MIRRORS-https://ghfast.top https://gh-proxy.com}
install_dir=${WEBMUX_INSTALL_DIR:-"${HOME:-}/.local/bin"}

# Built-in minisign Ed25519 release public keys (keep in sync with internal/update).
stable_pubkeys=${WEBMUX_STABLE_PUBKEYS:-RWQchIwF6KH3SuZyvOfqZZAoobboXxcEAyyLB3YptnKdLs7vUvxLHm4e}
dev_pubkeys=${WEBMUX_DEV_PUBKEYS:-RWSz8HBG69/zIIBeRUfEmjRf51vJmk7zueyTFeYAUemipmf+GU9cCiLr}

die() {
	printf 'webmux install: %s\n' "$*" >&2
	exit 1
}

note() {
	printf '%s\n' "$*"
}

[ -n "${HOME:-}" ] || [ -n "${WEBMUX_INSTALL_DIR:-}" ] || die "HOME is not set; set WEBMUX_INSTALL_DIR"
command -v curl >/dev/null 2>&1 || die "curl is required"
command -v uname >/dev/null 2>&1 || die "uname is required"
command -v mktemp >/dev/null 2>&1 || die "mktemp is required"
command -v awk >/dev/null 2>&1 || die "awk is required"

os=$(uname -s)
case $os in
Linux) os=linux ;;
Darwin) os=darwin ;;
MINGW* | MSYS* | CYGWIN*)
	die "this script runs on Linux and macOS only; on Windows download webmux-windows-<arch>.exe or the tray executable from $repo_base/releases/latest"
	;;
*)
	die "unsupported operating system: $os (need Linux or macOS; on Windows use the exe or WSL)"
	;;
esac

machine=$(uname -m)
case $machine in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) die "unsupported architecture: $machine (need amd64 or arm64)" ;;
esac

# Rosetta reports x86_64 on Apple Silicon; prefer the native build.
if [ "$os" = darwin ] && [ "$arch" = amd64 ]; then
	translated=$(sysctl -in sysctl.proc_translated 2>/dev/null || true)
	if [ "$translated" = 1 ]; then
		arch=arm64
	fi
fi

channel=${WEBMUX_CHANNEL:-}
case $channel in
'' | stable | dev) ;;
*) die "invalid WEBMUX_CHANNEL: $channel (use stable or dev)" ;;
esac

# version_kind prints "stable" for MAJOR.MINOR.PATCH, "dev" for
# MAJOR.MINOR.PATCH-dev.N (without leading zeros) and nothing for anything else.
version_kind() {
	case $1 in
	*[!0-9a-z.-]*) return 0 ;;
	esac
	printf '%s\n' "$1" | awk '
		/^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$/ { print "stable" }
		/^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-dev\.[1-9][0-9]*$/ { print "dev" }
	'
}

version=""
if [ -n "${WEBMUX_VERSION:-}" ]; then
	version=${WEBMUX_VERSION#v}
	case $(version_kind "$version") in
	stable)
		[ "$channel" != dev ] || die "WEBMUX_VERSION=$WEBMUX_VERSION is a release, not a development version; unset WEBMUX_CHANNEL=dev to install it"
		;;
	dev)
		[ "$channel" != stable ] || die "WEBMUX_VERSION=$WEBMUX_VERSION is a development version; it cannot be installed with WEBMUX_CHANNEL=stable"
		channel=dev
		;;
	*) die "invalid WEBMUX_VERSION: $WEBMUX_VERSION (use X.Y.Z or X.Y.Z-dev.N)" ;;
	esac
fi
channel=${channel:-stable}

release_label=""
if [ "$channel" = dev ]; then
	release_base="$repo_base/releases/download/dev"
elif [ -n "$version" ]; then
	release_base="$repo_base/releases/download/v$version"
	release_label="v$version"
else
	release_base="$repo_base/releases/latest/download"
	release_label="latest"
fi

pinned=""
if [ -n "${WEBMUX_SHA256:-}" ]; then
	pinned=$(printf '%s' "$WEBMUX_SHA256" | tr '[:upper:]' '[:lower:]')
	case $pinned in
	'' | *[!0-9a-f]*) die "WEBMUX_SHA256 must be a 64-character hex digest" ;;
	esac
	[ "${#pinned}" -eq 64 ] || die "WEBMUX_SHA256 must be a 64-character hex digest"
fi

digest() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	else
		die "need sha256sum or shasum to verify the download"
	fi
}

sums_digest() {
	awk -v name="$2" '{ sub(/\r$/, ""); n=$2; sub(/^\*/, "", n); if (n == name) { print $1; exit } }' "$1"
}

# dev_versions prints the version of every webmux-<X.Y.Z-dev.N>-<os>-<arch>
# line in the SHA256SUMS file $1 for os $2 and arch $3.
dev_versions() {
	awk -v suffix="-$2-$3" '{
		sub(/\r$/, "")
		n = $2
		sub(/^\*/, "", n)
		if (substr(n, 1, 7) != "webmux-" || length(n) <= 7 + length(suffix)) next
		if (substr(n, length(n) - length(suffix) + 1) != suffix) next
		v = substr(n, 8, length(n) - 7 - length(suffix))
		if (v ~ /^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)-dev\.[1-9][0-9]*$/) print v
	}' "$1"
}

download() {
	curl -fsSL --connect-timeout 10 --max-time "$3" --speed-limit 1024 --speed-time 20 -o "$2" "$1"
}

download_manifest() {
	curl -fsSL --connect-timeout 10 --max-time 60 --max-filesize 65536 --speed-limit 1024 --speed-time 20 -o "$2" "$1"
}

# parse_minisig validates the four-line minisign layout of $1 and writes the
# file-signature base64, trusted comment and global-signature base64 into $2.
parse_minisig() {
	awk -v d="$2" '
		{
			if (NR > 16 || length($0) > 4096) {
				bad = 1
				exit 1
			}
			sub(/\r$/, "")
			lines[++n] = $0
		}
		END {
			if (bad) exit 1
			while (n > 0 && lines[n] == "") n--
			if (n != 4) exit 1
			if (substr(lines[1], 1, 19) != "untrusted comment: ") exit 1
			if (!valid_b64(lines[2])) exit 1
			if (substr(lines[3], 1, 17) != "trusted comment: ") exit 1
			if (!valid_b64(lines[4])) exit 1
			f2 = d "/sig.b64"
			f3 = d "/trusted"
			f4 = d "/global.b64"
			printf "%s", lines[2] > f2
			close(f2)
			printf "%s", substr(lines[3], 18) > f3
			close(f3)
			printf "%s", lines[4] > f4
			close(f4)
		}
		function valid_b64(s) {
			return length(s) > 0 && length(s) % 4 == 0 &&
				s ~ /^[A-Za-z0-9+\/]+(=|==)?$/
		}
	' "$1"
}

verify_one_minisign() {
	case $3 in
	'' | *[!A-Za-z0-9+/=]*) return 1 ;;
	esac
	minisign -V -m "$1" -x "$2" -P "$3" >/dev/null 2>&1
}

# verify_one_openssl checks minisign's pure Ed25519 ("Ed") file signature and
# global (signature || trusted comment) signature using OpenSSL pkeyutl.
verify_one_openssl() {
	case $3 in
	'' | *[!A-Za-z0-9+/=]*) return 1 ;;
	esac
	printf '%s' "$3" | openssl base64 -d -A > "$2/pub.raw" 2>/dev/null || return 1
	[ "$(( $(wc -c < "$2/pub.raw") ))" -eq 42 ] || return 1
	[ "$(dd if="$2/pub.raw" bs=1 count=2 2>/dev/null | openssl base64 -A)" = "RWQ=" ] || return 1
	pub_id=$(dd if="$2/pub.raw" bs=1 skip=2 count=8 2>/dev/null | openssl base64 -A) || return 1

	openssl base64 -d -A -in "$2/sig.b64" -out "$2/sig.raw" 2>/dev/null || return 1
	[ "$(( $(wc -c < "$2/sig.raw") ))" -eq 74 ] || return 1
	[ "$(dd if="$2/sig.raw" bs=1 count=2 2>/dev/null | openssl base64 -A)" = "RWQ=" ] || return 1
	sig_id=$(dd if="$2/sig.raw" bs=1 skip=2 count=8 2>/dev/null | openssl base64 -A) || return 1
	[ "$sig_id" = "$pub_id" ] || return 1
	dd if="$2/sig.raw" bs=1 skip=10 count=64 of="$2/sig.bin" 2>/dev/null || return 1

	openssl base64 -d -A -in "$2/global.b64" -out "$2/global.bin" 2>/dev/null || return 1
	[ "$(( $(wc -c < "$2/global.bin") ))" -eq 64 ] || return 1

	{
		printf '\060\052\060\005\006\003\053\145\160\003\041\000'
		dd if="$2/pub.raw" bs=1 skip=10 count=32 2>/dev/null
	} | openssl base64 -A > "$2/spki.b64" || return 1
	{
		printf '%s\n' '-----BEGIN PUBLIC KEY-----'
		cat "$2/spki.b64"
		printf '\n%s\n' '-----END PUBLIC KEY-----'
	} > "$2/pub.pem" || return 1

	openssl pkeyutl -verify -pubin -inkey "$2/pub.pem" -rawin -in "$1" -sigfile "$2/sig.bin" >/dev/null 2>&1 || return 1
	cat "$2/sig.bin" "$2/trusted" > "$2/global.msg" || return 1
	openssl pkeyutl -verify -pubin -inkey "$2/pub.pem" -rawin -in "$2/global.msg" -sigfile "$2/global.bin" >/dev/null 2>&1
}

# detect_verifier probes whether minisign or OpenSSL on PATH can verify a
# known-good Ed25519 minisign signature and reject a tampered message. macOS
# ships LibreSSL as /usr/bin/openssl without Ed25519 -rawin support.
verifier=""
detect_verifier() {
	p_dir="$tmp_dir/probe"
	rm -rf "$p_dir"
	mkdir -p "$p_dir" || return 0
	printf 'ok\n' > "$p_dir/msg"
	printf 'no\n' > "$p_dir/bad"
	printf '%s\n' \
		'untrusted comment: signature from webmux release key 441638DF91637E22' \
		'RWQifmOR3zgWRJ/V1bYyhXiOKOlgtnUDgFGLOb9WqB9Q50qG+/XcxCLqJ+c6iji1nu9E0RqmWI8LedV5lWYAGfPEIFpcC+Um7gg=' \
		'trusted comment: webmux 0.0.0' \
		'kJDJbOHUuTWivgwZO8ZgSAQVkIGc9jiQRYjSZRE4MQwT8tm8LvrRdOTmkTwSJr4t1XD84yGdvaUj2L4Lm5iiCg==' \
		> "$p_dir/msg.minisig"
	p_pub=RWQifmOR3zgWRK8o81UerqVzwz4euISaPbJdRKiGufA+p2vU7/WSVaFB
	if parse_minisig "$p_dir/msg.minisig" "$p_dir"; then
		if command -v minisign >/dev/null 2>&1 &&
			verify_one_minisign "$p_dir/msg" "$p_dir/msg.minisig" "$p_pub" &&
			! verify_one_minisign "$p_dir/bad" "$p_dir/msg.minisig" "$p_pub"; then
			verifier=minisign
		elif command -v openssl >/dev/null 2>&1 &&
			command -v dd >/dev/null 2>&1 &&
			command -v wc >/dev/null 2>&1 &&
			command -v cat >/dev/null 2>&1 &&
			verify_one_openssl "$p_dir/msg" "$p_dir" "$p_pub" &&
			! verify_one_openssl "$p_dir/bad" "$p_dir" "$p_pub"; then
			verifier=openssl
		fi
	fi
	rm -rf "$p_dir"
}

sig_version=""
verify_error=""
verify_manifest() {
	v_dir="$tmp_dir/verify"
	rm -rf "$v_dir"
	mkdir -p "$v_dir" || {
		verify_error="cannot create verification workspace"
		return 1
	}
	if ! parse_minisig "$2" "$v_dir"; then
		rm -rf "$v_dir"
		verify_error="SHA256SUMS.minisig format is invalid"
		return 1
	fi
	if [ "$channel" = dev ]; then
		key_list=$dev_pubkeys
	else
		key_list=$stable_pubkeys
	fi
	matched=0
	for pubkey in $key_list; do
		if [ "$verifier" = minisign ]; then
			if verify_one_minisign "$1" "$2" "$pubkey"; then
				matched=1
				break
			fi
		elif verify_one_openssl "$1" "$v_dir" "$pubkey"; then
			matched=1
			break
		fi
	done
	if [ "$matched" -ne 1 ]; then
		rm -rf "$v_dir"
		verify_error="SHA256SUMS signature verification failed"
		return 1
	fi
	trusted=$(cat "$v_dir/trusted")
	rm -rf "$v_dir"
	case $trusted in
	'webmux '*) sig_version=${trusted#webmux } ;;
	*)
		verify_error="SHA256SUMS.minisig has invalid trusted comment"
		return 1
		;;
	esac
	if [ "$(version_kind "$sig_version")" != "$channel" ]; then
		verify_error="SHA256SUMS.minisig version $sig_version does not match channel $channel"
		return 1
	fi
	return 0
}

dev_error=""
resolve_dev_asset() {
	build=$(dev_versions "$1" "$os" "$arch")
	case $build in
	'')
		dev_error="SHA256SUMS lists no webmux-<X.Y.Z-dev.N>-$os-$arch"
		return 1
		;;
	*'
'*)
		dev_error="SHA256SUMS lists several webmux-<X.Y.Z-dev.N>-$os-$arch"
		return 1
		;;
	esac
	if [ -n "${2:-}" ] && [ "$build" != "$2" ]; then
		dev_error="SHA256SUMS.minisig is for $2, SHA256SUMS lists $build"
		return 1
	fi
	if [ -n "$version" ] && [ "$version" != "$build" ]; then
		dev_error="development version $version is no longer available; the current development version is $build"
		return 1
	fi
	asset="webmux-$build-$os-$arch"
	source_label=$build
	return 0
}

extract_expected() {
	expected=$(sums_digest "$1" "$2")
	case $expected in
	'' | *[!0-9a-f]*) return 1 ;;
	esac
	[ "${#expected}" -eq 64 ]
}

mkdir -p "$install_dir" 2>/dev/null || die "cannot create install directory: $install_dir"
[ -d "$install_dir" ] || die "install path is not a directory: $install_dir"
[ -w "$install_dir" ] || die "install directory is not writable: $install_dir (set WEBMUX_INSTALL_DIR)"

tmp_dir=$(mktemp -d "$install_dir/.webmux-install.XXXXXX") || die "cannot create a temporary directory in $install_dir"
cleanup() {
	rm -rf "$tmp_dir"
}
trap cleanup EXIT
trap 'exit 130' HUP INT TERM

if [ -z "$pinned" ]; then
	detect_verifier
fi

target="$install_dir/webmux"
set -- "$release_base"
for mirror in $mirrors; do
	mirror=${mirror%/}
	[ -n "$mirror" ] || continue
	set -- "$@" "$mirror/$release_base"
done

trusted_expected=""
trusted_asset=""
trusted_label=""

failures=""
for source in "$@"; do
	sums_path="$tmp_dir/SHA256SUMS"
	sig_path="$tmp_dir/SHA256SUMS.minisig"
	rm -f "$tmp_dir"/webmux-* "$sums_path" "$sig_path"

	asset="webmux-$os-$arch"
	source_label=$release_label
	expected=""

	if [ -n "$pinned" ]; then
		if [ "$channel" = dev ]; then
			if ! download_manifest "$source/SHA256SUMS" "$sums_path"; then
				failures="$failures
  $source: SHA256SUMS download failed"
				continue
			fi
			if ! resolve_dev_asset "$sums_path"; then
				failures="$failures
  $source: $dev_error"
				continue
			fi
		fi
		expected=$pinned
	elif [ -n "$verifier" ]; then
		if ! download_manifest "$source/SHA256SUMS" "$sums_path"; then
			failures="$failures
  $source: SHA256SUMS download failed"
			continue
		fi
		if ! download_manifest "$source/SHA256SUMS.minisig" "$sig_path"; then
			if [ "$channel" = stable ] && [ -n "$version" ]; then
				failures="$failures
  $source: SHA256SUMS.minisig download failed (releases older than v0.11.0 are unsigned; set WEBMUX_SHA256 to pin them)"
			else
				failures="$failures
  $source: SHA256SUMS.minisig download failed"
			fi
			continue
		fi
		if ! verify_manifest "$sums_path" "$sig_path"; then
			failures="$failures
  $source: $verify_error"
			continue
		fi
		if [ "$channel" = dev ]; then
			if ! resolve_dev_asset "$sums_path" "$sig_version"; then
				failures="$failures
  $source: $dev_error"
				continue
			fi
		else
			if [ -n "$version" ] && [ "$sig_version" != "$version" ]; then
				failures="$failures
  $source: SHA256SUMS.minisig is for v$sig_version, expected v$version"
				continue
			fi
			source_label="v$sig_version"
		fi
		if ! extract_expected "$sums_path" "$asset"; then
			failures="$failures
  $source: SHA256SUMS has no valid digest for $asset"
			continue
		fi
	elif [ "$repo_base" = "$canonical_repo" ] && [ "$source" = "$release_base" ]; then
		if ! download_manifest "$source/SHA256SUMS" "$sums_path"; then
			failures="$failures
  $source: SHA256SUMS download failed"
			continue
		fi
		if [ "$channel" = dev ]; then
			if ! resolve_dev_asset "$sums_path"; then
				failures="$failures
  $source: $dev_error"
				continue
			fi
		fi
		if ! extract_expected "$sums_path" "$asset"; then
			failures="$failures
  $source: SHA256SUMS has no valid digest for $asset"
			continue
		fi
		trusted_expected=$expected
		trusted_asset=$asset
		trusted_label=$source_label
	elif [ -n "$trusted_expected" ]; then
		asset=$trusted_asset
		expected=$trusted_expected
		source_label=$trusted_label
	else
		failures="$failures
  $source: refusing unverified SHA256SUMS from non-GitHub source (install minisign or OpenSSL with Ed25519 support, or set WEBMUX_SHA256)"
		continue
	fi

	asset_path="$tmp_dir/$asset"
	if ! download "$source/$asset" "$asset_path" 600; then
		failures="$failures
  $source: binary download failed"
		continue
	fi

	actual=$(digest "$asset_path")
	if [ "$actual" != "$expected" ]; then
		failures="$failures
  $source: sha256 mismatch (expected $expected, got $actual)"
		continue
	fi

	chmod 755 "$asset_path" || {
		failures="$failures
  $source: cannot make the download executable"
		continue
	}
	if ! mv -f "$asset_path" "$target"; then
		failures="$failures
  $source: cannot write $target"
		continue
	fi

	host=${source#*://}
	host=${host%%/*}
	note "Installed webmux ($source_label) to $target (from $host)."
	case ":${PATH:-}:" in
	*":$install_dir:"*) ;;
	*) note "Note: $install_dir is not on PATH; add it or run $target directly." ;;
	esac
	note ""
	note "Start WebMux:"
	note "  $target"
	note "Install it as a user service:"
	note "  $target service install"
	exit 0
done

printf 'webmux install: all download sources failed:%s\n' "$failures" >&2
exit 1
