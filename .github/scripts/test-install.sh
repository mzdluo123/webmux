#!/bin/sh
# Exercise install.sh against a local release fixture.
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
work=$(mktemp -d)
server_pid=
cleanup() {
	if [ -n "$server_pid" ]; then
		kill "$server_pid" 2>/dev/null || true
	fi
	rm -rf "$work"
}
trap cleanup EXIT HUP INT TERM

fail() {
	printf 'test-install: %s\n' "$*" >&2
	exit 1
}

# macOS ships shasum rather than GNU sha256sum.
digest() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

sh -n "$repo/install.sh" || fail "install.sh has a syntax error"
sh -n "$0" || fail "test-install.sh has a syntax error"

require_verifier() {
	p_dir=$work/verifier-probe
	mkdir -p "$p_dir"
	printf 'ok\n' > "$p_dir/msg"
	printf '%s\n' \
		'untrusted comment: signature from webmux release key 441638DF91637E22' \
		'RWQifmOR3zgWRJ/V1bYyhXiOKOlgtnUDgFGLOb9WqB9Q50qG+/XcxCLqJ+c6iji1nu9E0RqmWI8LedV5lWYAGfPEIFpcC+Um7gg=' \
		'trusted comment: webmux 0.0.0' \
		'kJDJbOHUuTWivgwZO8ZgSAQVkIGc9jiQRYjSZRE4MQwT8tm8LvrRdOTmkTwSJr4t1XD84yGdvaUj2L4Lm5iiCg==' \
		> "$p_dir/msg.minisig"
	p_pub=RWQifmOR3zgWRK8o81UerqVzwz4euISaPbJdRKiGufA+p2vU7/WSVaFB
	if command -v minisign >/dev/null 2>&1 &&
		minisign -V -m "$p_dir/msg" -x "$p_dir/msg.minisig" -P "$p_pub" >/dev/null 2>&1; then
		rm -rf "$p_dir"
		return 0
	fi
	if command -v openssl >/dev/null 2>&1 &&
		command -v dd >/dev/null 2>&1 &&
		command -v cat >/dev/null 2>&1 &&
		printf '%s' "$p_pub" | openssl base64 -d -A > "$p_dir/pub.raw" 2>/dev/null &&
		{
			printf '\060\052\060\005\006\003\053\145\160\003\041\000'
			dd if="$p_dir/pub.raw" bs=1 skip=10 count=32 2>/dev/null
		} | openssl base64 -A > "$p_dir/spki.b64" &&
		{
			printf '%s\n' '-----BEGIN PUBLIC KEY-----'
			cat "$p_dir/spki.b64"
			printf '\n%s\n' '-----END PUBLIC KEY-----'
		} > "$p_dir/pub.pem" &&
		printf '%s' 'RWQifmOR3zgWRJ/V1bYyhXiOKOlgtnUDgFGLOb9WqB9Q50qG+/XcxCLqJ+c6iji1nu9E0RqmWI8LedV5lWYAGfPEIFpcC+Um7gg=' |
			openssl base64 -d -A > "$p_dir/sig.raw" 2>/dev/null &&
		dd if="$p_dir/sig.raw" bs=1 skip=10 count=64 of="$p_dir/sig.bin" 2>/dev/null &&
		openssl pkeyutl -verify -pubin -inkey "$p_dir/pub.pem" -rawin \
			-in "$p_dir/msg" -sigfile "$p_dir/sig.bin" >/dev/null 2>&1; then
		rm -rf "$p_dir"
		return 0
	fi
	fail "install minisign or an OpenSSL with Ed25519 support (for example: brew install minisign)"
}
require_verifier

# Build releasesign once so fixtures carry real minisign Ed25519 signatures.
signer=$work/releasesign
(cd "$repo" && go build -o "$signer" ./cmd/internal/releasesign) || fail "cannot build releasesign"

stable_pub_out=$("$signer" keygen -secret "$work/stable.sec")
stable_pub=$(printf '%s\n' "$stable_pub_out" | awk 'END { print }')
stable_sec=$(cat "$work/stable.sec")

dev_pub_out=$("$signer" keygen -secret "$work/dev.sec")
dev_pub=$(printf '%s\n' "$dev_pub_out" | awk 'END { print }')
dev_sec=$(cat "$work/dev.sec")

attacker_pub_out=$("$signer" keygen -secret "$work/attacker.sec")
attacker_pub=$(printf '%s\n' "$attacker_pub_out" | awk 'END { print }')
attacker_sec=$(cat "$work/attacker.sec")
[ "$attacker_pub" != "$stable_pub" ] || fail "duplicate test keys"

sign_sums() {
	dir=$1
	ver=$2
	WEBMUX_RELEASE_SIGNING_KEY="$stable_sec" WEBMUX_DEV_SIGNING_KEY="$dev_sec" \
		"$signer" sign -version "$ver" "$dir/SHA256SUMS"
}

write_sums() {
	(
		cd "$1"
		for file in webmux-*; do
			printf '%s  %s\n' "$(digest "$file")" "$file"
		done > SHA256SUMS
	)
}

www=$work/www
mkdir -p "$www/releases/latest/download"
for os in linux darwin; do
	for arch in amd64 arm64; do
		asset=webmux-$os-$arch
		printf '#!/bin/sh\n# %s latest\nprintf "webmux 0.2.0\\n"\n' "$asset" > "$www/releases/latest/download/$asset"
		chmod 755 "$www/releases/latest/download/$asset"
	done
done
write_sums "$www/releases/latest/download"
sign_sums "$www/releases/latest/download" 0.2.0

mkdir -p "$www/releases/download/v0.2.0"
for os in linux darwin; do
	for arch in amd64 arm64; do
		asset=webmux-$os-$arch
		printf '#!/bin/sh\n# %s pinned\nprintf "webmux 0.2.0\\n"\n' "$asset" > "$www/releases/download/v0.2.0/$asset"
		chmod 755 "$www/releases/download/v0.2.0/$asset"
	done
done
write_sums "$www/releases/download/v0.2.0"
sign_sums "$www/releases/download/v0.2.0" 0.2.0

# Validly signed SHA256SUMS whose binary bytes were corrupted in transit.
mkdir -p "$www/bad/releases/latest/download"
printf 'bad binary\n' > "$www/bad/releases/latest/download/webmux-linux-amd64"
printf '%064d  webmux-linux-amd64\n' 0 > "$www/bad/releases/latest/download/SHA256SUMS"
sign_sums "$www/bad/releases/latest/download" 0.2.0

# The rolling dev Release mid-publish: the current build 0.10.3-dev.4 and
# the outgoing 0.10.3-dev.3, whose files are not yet deleted but are no
# longer in SHA256SUMS.
dev_dir=$www/releases/download/dev
mkdir -p "$dev_dir"
for build in 0.10.3-dev.3 0.10.3-dev.4; do
	for asset in linux-amd64 linux-arm64 darwin-arm64 tray-windows-amd64.exe; do
		asset=webmux-$build-$asset
		printf '#!/bin/sh\n# %s dev\nprintf "webmux %s\\n"\n' "$asset" "$build" > "$dev_dir/$asset"
		chmod 755 "$dev_dir/$asset"
	done
done
# Signed development releases also list standalone tarballs and frp notices;
# install.sh must still resolve the single matching bare WebMux file.
for extra in linux-amd64.tar.gz frpc-pins.txt frpc-LICENSE.txt; do
	printf '%s\n' "$extra" > "$dev_dir/webmux-0.10.3-dev.4-$extra"
done
cp "$repo/install.sh" "$dev_dir/install.sh"
(
	cd "$dev_dir"
	for file in webmux-0.10.3-dev.4-* install.sh; do
		printf '%s  %s\n' "$(digest "$file")" "$file"
	done > SHA256SUMS
)
sign_sums "$dev_dir" 0.10.3-dev.4

# A dev SHA256SUMS listing two builds for linux-amd64 and none for linux-arm64.
mkdir -p "$www/ambiguous/releases/download/dev"
cp "$dev_dir"/webmux-*-linux-amd64 "$www/ambiguous/releases/download/dev/"
write_sums "$www/ambiguous/releases/download/dev"
sign_sums "$www/ambiguous/releases/download/dev" 0.10.3-dev.4

# A dev SHA256SUMS whose single asset version disagrees with the signed version.
mkdir -p "$www/dev-mismatch/releases/download/dev"
cp "$dev_dir/webmux-0.10.3-dev.3-linux-amd64" "$www/dev-mismatch/releases/download/dev/"
write_sums "$www/dev-mismatch/releases/download/dev"
sign_sums "$www/dev-mismatch/releases/download/dev" 0.10.3-dev.4

port=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')
python3 -m http.server "$port" --bind 127.0.0.1 --directory "$www" >/dev/null 2>&1 &
server_pid=$!
server=http://127.0.0.1:$port
tries=0
while ! curl -fsS "$server/" >/dev/null 2>&1; do
	tries=$((tries + 1))
	[ "$tries" -lt 100 ] || fail "local fixture server did not start"
	sleep 0.1
done

mkdir -p "$work/home" "$work/out"

fake_uname() {
	dir=$1
	system=$2
	machine=$3
	mkdir -p "$dir"
	cat > "$dir/uname" <<EOF
#!/bin/sh
case "\$1" in
-s) printf '%s\\n' "$system" ;;
-m) printf '%s\\n' "$machine" ;;
*) printf '%s\\n' "$system" ;;
esac
EOF
	chmod 755 "$dir/uname"
}

run_install() {
	uname_dir=$1
	shift
	env HOME="$work/home" PATH="$uname_dir:$PATH" WEBMUX_INSTALL_DIR="$work/out" \
		WEBMUX_BASE_URL="$base" WEBMUX_MIRRORS="$mirrors" \
		WEBMUX_STABLE_PUBKEYS="$stable_pub" WEBMUX_DEV_PUBKEYS="$dev_pub" "$@" \
		sh "$repo/install.sh"
}

reset_out() {
	rm -rf "$work/out"
	mkdir -p "$work/out"
}

base=$server
mirrors=

fake_uname "$work/uname-linux-amd64" Linux x86_64
reset_out
run_install "$work/uname-linux-amd64" >"$work/linux.log" 2>&1 || {
	cat "$work/linux.log"
	fail "Linux amd64 installation failed"
}
[ -x "$work/out/webmux" ] || fail "Linux amd64 binary was not installed"
grep -q 'webmux-linux-amd64 latest' "$work/out/webmux" || fail "Linux amd64 selected the wrong asset"
grep -qF '(v0.2.0)' "$work/linux.log" || fail "Linux amd64 did not report the verified release version"

# Hide sha256sum so install.sh has to fall back to shasum while still verifying minisig.
if command -v shasum >/dev/null 2>&1; then
	shasum_only=$work/shasum-only
	mkdir -p "$shasum_only"
	for tool in sh curl mktemp awk tr mkdir mv chmod rm dd wc cat; do
		ln -s "$(command -v "$tool")" "$shasum_only/$tool"
	done
	for opt_tool in minisign openssl; do
		if command -v "$opt_tool" >/dev/null 2>&1; then
			ln -s "$(command -v "$opt_tool")" "$shasum_only/$opt_tool"
		fi
	done
	ln -s "$(command -v shasum)" "$shasum_only/shasum"
	fake_uname "$shasum_only" Linux x86_64
	reset_out
	env HOME="$work/home" PATH="$shasum_only" WEBMUX_INSTALL_DIR="$work/out" \
		WEBMUX_BASE_URL="$base" WEBMUX_MIRRORS= \
		WEBMUX_STABLE_PUBKEYS="$stable_pub" WEBMUX_DEV_PUBKEYS="$dev_pub" \
		sh "$repo/install.sh" >"$work/shasum.log" 2>&1 || {
		cat "$work/shasum.log"
		fail "shasum fallback installation failed"
	}
	grep -q 'webmux-linux-amd64 latest' "$work/out/webmux" || fail "shasum fallback installed the wrong asset"
fi

# Exercise the minisign CLI verifier path with a wrapper backed by releasesign.
fake_minisign_dir=$work/fake-minisign
mkdir -p "$fake_minisign_dir"
cat > "$fake_minisign_dir/minisign" <<EOF
#!/bin/sh
set -eu
msg=""
sig=""
pub=""
while [ "\$#" -gt 0 ]; do
	case "\$1" in
	-V) shift ;;
	-m) msg=\$2; shift 2 ;;
	-x) sig=\$2; shift 2 ;;
	-P) pub=\$2; shift 2 ;;
	*) exit 2 ;;
	esac
done
tmp=\$(mktemp -d)
trap 'rm -rf "\$tmp"' EXIT
cp "\$msg" "\$tmp/file"
cp "\$sig" "\$tmp/file.minisig"
ver=\$(awk 'sub(/^trusted comment: webmux /, "") { sub(/\r\$/, ""); print; exit }' "\$sig")
exec "$signer" verify -version "\$ver" -pubkey "\$pub" "\$tmp/file" >/dev/null
EOF
chmod 755 "$fake_minisign_dir/minisign"
fake_uname "$fake_minisign_dir" Linux x86_64
reset_out
run_install "$fake_minisign_dir" >"$work/minisign-cli.log" 2>&1 || {
	cat "$work/minisign-cli.log"
	fail "minisign CLI path installation failed"
}
grep -q 'webmux-linux-amd64 latest' "$work/out/webmux" || fail "minisign CLI path installed the wrong asset"

fake_uname "$work/uname-darwin-arm64" Darwin arm64
reset_out
run_install "$work/uname-darwin-arm64" >"$work/darwin.log" 2>&1 || {
	cat "$work/darwin.log"
	fail "Darwin arm64 installation failed"
}
grep -q 'webmux-darwin-arm64 latest' "$work/out/webmux" || fail "Darwin arm64 selected the wrong asset"

# Rosetta reports x86_64; sysctl.proc_translated=1 must select arm64.
fake_uname "$work/uname-darwin-rosetta" Darwin x86_64
cat > "$work/uname-darwin-rosetta/sysctl" <<'EOF'
#!/bin/sh
printf '1\n'
EOF
chmod 755 "$work/uname-darwin-rosetta/sysctl"
reset_out
run_install "$work/uname-darwin-rosetta" >"$work/rosetta.log" 2>&1 || {
	cat "$work/rosetta.log"
	fail "Rosetta installation failed"
}
grep -q 'webmux-darwin-arm64 latest' "$work/out/webmux" || fail "Rosetta did not select arm64"

base=$server
mirrors=
fake_uname "$work/uname-linux-pinned" Linux x86_64
reset_out
run_install "$work/uname-linux-pinned" WEBMUX_VERSION=v0.2.0 >"$work/pinned.log" 2>&1 || {
	cat "$work/pinned.log"
	fail "pinned version installation failed"
}
grep -q 'webmux-linux-amd64 pinned' "$work/out/webmux" || fail "pinned version selected the wrong release"

# A pinned digest must work even when the source has no SHA256SUMS or signature.
mkdir -p "$www/nosums/releases/latest/download"
cp "$www/releases/latest/download/webmux-linux-amd64" "$www/nosums/releases/latest/download/"
pinned_sum=$(digest "$www/nosums/releases/latest/download/webmux-linux-amd64")
base=$server/nosums
reset_out
run_install "$work/uname-linux-pinned" WEBMUX_SHA256="$pinned_sum" >"$work/pinned-sum.log" 2>&1 || {
	cat "$work/pinned-sum.log"
	fail "pinned digest installation failed"
}
grep -q 'webmux-linux-amd64 latest' "$work/out/webmux" || fail "pinned digest installed the wrong asset"

# A checksum mismatch must fail and leave the previous binary untouched.
base=$server/bad
mirrors=
reset_out
printf 'old binary\n' > "$work/out/webmux"
chmod 755 "$work/out/webmux"
if run_install "$work/uname-linux-amd64" >"$work/bad.log" 2>&1; then
	cat "$work/bad.log"
	fail "checksum mismatch unexpectedly succeeded"
fi
[ "$(cat "$work/out/webmux")" = "old binary" ] || fail "checksum mismatch replaced the existing binary"
if find "$work/out" -maxdepth 1 -name '.webmux-install.*' | grep -q .; then
	fail "checksum mismatch left a temporary directory"
fi

# The first source 404s; the gh-proxy style mirror prefix must supply a validly signed release.
base=$server/missing
mirrors=$server/mirror
mirror_dir="$www/mirror/http:/127.0.0.1:$port/missing/releases/latest/download"
mkdir -p "$mirror_dir"
for entry in "$www/releases/latest/download"/*; do
	cp "$entry" "$mirror_dir/"
done
reset_out
run_install "$work/uname-linux-amd64" >"$work/mirror.log" 2>&1 || {
	cat "$work/mirror.log"
	fail "mirror fallback installation failed"
}
grep -q 'webmux-linux-amd64 latest' "$work/out/webmux" || fail "mirror fallback installed the wrong asset"

# expect_failure LOG MESSAGE UNAME_DIR [VAR=value...]: install.sh must fail,
# report MESSAGE and leave the installed binary and directory as they were.
expect_failure() {
	log=$1
	message=$2
	shift 2
	reset_out
	printf 'old binary\n' > "$work/out/webmux"
	chmod 755 "$work/out/webmux"
	if run_install "$@" >"$log" 2>&1; then
		fail "$log: installation unexpectedly succeeded"
	fi
	grep -qF -- "$message" "$log" || fail "$log: expected '$message', got: $(cat "$log")"
	[ "$(cat "$work/out/webmux")" = "old binary" ] || fail "$log: replaced the installed binary"
	[ "$(ls -A "$work/out")" = webmux ] || fail "$log: left files in the install directory"
}

# P0-01 regression: a malicious mirror serving a trojan binary and matching
# unsigned SHA256SUMS must be rejected.
forged_unsigned_dir="$www/forged-unsigned/http:/127.0.0.1:$port/missing/releases/latest/download"
mkdir -p "$forged_unsigned_dir"
printf '#!/bin/sh\nprintf "trojan\\n"\n' > "$forged_unsigned_dir/webmux-linux-amd64"
write_sums "$forged_unsigned_dir"
base=$server/missing
mirrors=$server/forged-unsigned
expect_failure "$work/forged-unsigned.log" 'SHA256SUMS.minisig download failed' \
	"$work/uname-linux-amd64"

# Malicious mirror serving forged SHA256SUMS paired with the real release's signature.
forged_sig_dir="$www/forged-sig/http:/127.0.0.1:$port/missing/releases/latest/download"
mkdir -p "$forged_sig_dir"
cp "$forged_unsigned_dir/webmux-linux-amd64" "$forged_sig_dir/"
cp "$forged_unsigned_dir/SHA256SUMS" "$forged_sig_dir/"
cp "$www/releases/latest/download/SHA256SUMS.minisig" "$forged_sig_dir/"
mirrors=$server/forged-sig
expect_failure "$work/forged-sig.log" 'SHA256SUMS signature verification failed' \
	"$work/uname-linux-amd64"

# Malicious mirror signing forged SHA256SUMS with an attacker's own Ed25519 key.
attacker_dir="$www/attacker-signed/http:/127.0.0.1:$port/missing/releases/latest/download"
mkdir -p "$attacker_dir"
cp "$forged_unsigned_dir/webmux-linux-amd64" "$attacker_dir/"
cp "$forged_unsigned_dir/SHA256SUMS" "$attacker_dir/"
WEBMUX_RELEASE_SIGNING_KEY="$attacker_sec" "$signer" sign -version 0.2.0 "$attacker_dir/SHA256SUMS"
mirrors=$server/attacker-signed
expect_failure "$work/attacker-key.log" 'SHA256SUMS signature verification failed' \
	"$work/uname-linux-amd64"

# Built-in production keys (without WEBMUX_STABLE_PUBKEYS override) must reject test keys.
reset_out
printf 'old binary\n' > "$work/out/webmux"
chmod 755 "$work/out/webmux"
if env HOME="$work/home" PATH="$work/uname-linux-amd64:$PATH" WEBMUX_INSTALL_DIR="$work/out" \
	WEBMUX_BASE_URL="$server" WEBMUX_MIRRORS= sh "$repo/install.sh" >"$work/builtin-keys.log" 2>&1; then
	fail "built-in release keys unexpectedly accepted a test signature"
fi
grep -qF 'SHA256SUMS signature verification failed' "$work/builtin-keys.log" || {
	fail "built-in key check failed for unexpected reason: $(cat "$work/builtin-keys.log")"
}
[ "$(cat "$work/out/webmux")" = "old binary" ] || fail "built-in key rejection replaced binary"

# Tampered trusted comment must fail global Ed25519 signature verification.
tampered_comment_dir="$www/tampered-comment/http:/127.0.0.1:$port/missing/releases/latest/download"
mkdir -p "$tampered_comment_dir"
for entry in "$www/releases/latest/download"/*; do
	cp "$entry" "$tampered_comment_dir/"
done
awk '{ if (NR == 3) print "trusted comment: webmux 9.9.9"; else print }' \
	"$www/releases/latest/download/SHA256SUMS.minisig" > "$tampered_comment_dir/SHA256SUMS.minisig"
base=$server/missing
mirrors=$server/tampered-comment
expect_failure "$work/tampered-comment.log" 'SHA256SUMS signature verification failed' \
	"$work/uname-linux-amd64"

# Dev key signature must never be trusted on the stable channel.
dev_on_stable_dir="$www/dev-on-stable/http:/127.0.0.1:$port/missing/releases/latest/download"
mkdir -p "$dev_on_stable_dir"
cp "$dev_dir/SHA256SUMS" "$dev_dir/SHA256SUMS.minisig" "$dev_on_stable_dir/"
mirrors=$server/dev-on-stable
expect_failure "$work/dev-on-stable.log" 'SHA256SUMS signature verification failed' \
	"$work/uname-linux-amd64"

# Replay of a validly signed 0.2.0 manifest under a requested v0.3.0 URL must fail.
replay_dir="$www/replay/releases/download/v0.3.0"
mkdir -p "$replay_dir"
for entry in "$www/releases/download/v0.2.0"/*; do
	cp "$entry" "$replay_dir/"
done
base=$server/replay
mirrors=
expect_failure "$work/version-replay.log" 'SHA256SUMS.minisig is for v0.2.0, expected v0.3.0' \
	"$work/uname-linux-amd64" WEBMUX_VERSION=0.3.0

# Pinning a pre-v0.11.0 release without SHA256SUMS.minisig must explain the WEBMUX_SHA256 workaround.
unsigned_tag_dir="$www/unsigned-tag/releases/download/v0.10.2"
mkdir -p "$unsigned_tag_dir"
cp "$www/releases/download/v0.2.0/webmux-linux-amd64" "$www/releases/download/v0.2.0/SHA256SUMS" "$unsigned_tag_dir/"
base=$server/unsigned-tag
mirrors=
expect_failure "$work/unsigned-tag.log" 'releases older than v0.11.0 are unsigned; set WEBMUX_SHA256 to pin them' \
	"$work/uname-linux-amd64" WEBMUX_VERSION=0.10.2

# A poisoned first mirror must be skipped in favour of a validly signed second mirror.
base=$server/missing
mirrors="$server/forged-sig $server/mirror"
reset_out
run_install "$work/uname-linux-amd64" >"$work/mirror-failover.log" 2>&1 || {
	cat "$work/mirror-failover.log"
	fail "failover past poisoned mirror failed"
}
grep -q 'webmux-linux-amd64 latest' "$work/out/webmux" || fail "failover installed the wrong asset"

# Without minisign or Ed25519-capable OpenSSL (e.g. stock macOS LibreSSL),
# non-GitHub / mirror sources must be refused unless WEBMUX_SHA256 is set.
no_verifier=$work/no-verifier
mkdir -p "$no_verifier"
for tool in sh curl mktemp awk tr mkdir mv chmod rm dirname; do
	ln -s "$(command -v "$tool")" "$no_verifier/$tool"
done
if command -v sha256sum >/dev/null 2>&1; then
	ln -s "$(command -v sha256sum)" "$no_verifier/sha256sum"
else
	ln -s "$(command -v shasum)" "$no_verifier/shasum"
fi
fake_uname "$no_verifier" Linux x86_64

reset_out
printf 'old binary\n' > "$work/out/webmux"
chmod 755 "$work/out/webmux"
if env HOME="$work/home" PATH="$no_verifier" WEBMUX_INSTALL_DIR="$work/out" \
	WEBMUX_BASE_URL="$server/missing" WEBMUX_MIRRORS="$server/forged-unsigned" \
	sh "$repo/install.sh" >"$work/no-verifier-mirror.log" 2>&1; then
	fail "mirror without verifier or WEBMUX_SHA256 unexpectedly succeeded"
fi
grep -qF 'refusing unverified SHA256SUMS from non-GitHub source' "$work/no-verifier-mirror.log" || {
	fail "unexpected no-verifier error: $(cat "$work/no-verifier-mirror.log")"
}
[ "$(cat "$work/out/webmux")" = "old binary" ] || fail "no-verifier failure replaced binary"

# test-install.sh itself must report an actionable prerequisite error when no verifier is on PATH.
if env PATH="$no_verifier" sh "$0" >"$work/preflight-missing-verifier.log" 2>&1; then
	fail "test-install.sh unexpectedly passed without minisign or Ed25519 OpenSSL"
fi
grep -qF 'install minisign or an OpenSSL with Ed25519 support' "$work/preflight-missing-verifier.log" || {
	fail "unexpected preflight error: $(cat "$work/preflight-missing-verifier.log")"
}

# With WEBMUX_SHA256 set, mirror fallback without a local signature verifier is safe and allowed.
reset_out
env HOME="$work/home" PATH="$no_verifier" WEBMUX_INSTALL_DIR="$work/out" \
	WEBMUX_BASE_URL="$server/missing" WEBMUX_MIRRORS="$server/mirror" \
	WEBMUX_SHA256="$pinned_sum" sh "$repo/install.sh" >"$work/no-verifier-pinned.log" 2>&1 || {
	cat "$work/no-verifier-pinned.log"
	fail "mirror with WEBMUX_SHA256 and no verifier failed"
}
grep -q 'webmux-linux-amd64 latest' "$work/out/webmux" || fail "no-verifier pinned install chose wrong asset"

# Without a local signature verifier, canonical GitHub may supply SHA256SUMS and
# a mirror may supply only a binary matching that GitHub digest (rejecting trojans).
gh_bad_dir="$www/gh-forged/https:/github.com/mzdluo123/webmux/releases/latest/download"
mkdir -p "$gh_bad_dir"
cp "$forged_unsigned_dir/webmux-linux-amd64" "$forged_unsigned_dir/SHA256SUMS" "$gh_bad_dir/"
gh_good_dir="$www/gh-good/https:/github.com/mzdluo123/webmux/releases/latest/download"
mkdir -p "$gh_good_dir"
cp "$www/releases/latest/download/webmux-linux-amd64" "$gh_good_dir/"

real_curl=$(command -v curl)
no_verifier_gh=$work/no-verifier-gh
mkdir -p "$no_verifier_gh"
for entry in "$no_verifier"/*; do
	[ "$(basename "$entry")" = curl ] && continue
	ln -s "$entry" "$no_verifier_gh/$(basename "$entry")"
done
cat > "$no_verifier_gh/curl" <<EOF
#!/bin/sh
url=""
for arg in "\$@"; do
	url=\$arg
done
case "\$url" in
https://github.com/mzdluo123/webmux/releases/latest/download/SHA256SUMS)
	[ "\${FAKE_GITHUB_SUMS:-1}" = 1 ] || exit 22
	mapped="$server/releases/latest/download/SHA256SUMS"
	;;
https://github.com/*)
	exit 22
	;;
*)
	mapped=\$url
	;;
esac
# Replace final URL argument with mapped URL.
out=""
while [ "\$#" -gt 1 ]; do
	out="\$out '\$1'"
	shift
done
eval "exec '$real_curl' \$out '\$mapped'"
EOF
chmod 755 "$no_verifier_gh/curl"

reset_out
env HOME="$work/home" PATH="$no_verifier_gh" WEBMUX_INSTALL_DIR="$work/out" \
	WEBMUX_MIRRORS="$server/gh-forged $server/gh-good" \
	sh "$repo/install.sh" >"$work/no-verifier-gh-sums.log" 2>&1 || {
	cat "$work/no-verifier-gh-sums.log"
	fail "canonical GitHub SHA256SUMS + verified mirror binary failed"
}
grep -q 'webmux-linux-amd64 latest' "$work/out/webmux" || fail "canonical GitHub SHA256SUMS + mirror binary installed wrong asset"

reset_out
printf 'old binary\n' > "$work/out/webmux"
chmod 755 "$work/out/webmux"
if env HOME="$work/home" PATH="$no_verifier_gh" FAKE_GITHUB_SUMS=0 WEBMUX_INSTALL_DIR="$work/out" \
	WEBMUX_MIRRORS="$server/gh-forged $server/gh-good" \
	sh "$repo/install.sh" >"$work/no-verifier-gh-down.log" 2>&1; then
	fail "canonical GitHub unreachable without verifier unexpectedly fell back to unsigned mirror"
fi
grep -qF 'refusing unverified SHA256SUMS from non-GitHub source' "$work/no-verifier-gh-down.log" || {
	fail "unexpected error when canonical GitHub is unreachable: $(cat "$work/no-verifier-gh-down.log")"
}
[ "$(cat "$work/out/webmux")" = "old binary" ] || fail "canonical GitHub unreachable replaced binary"

base=$server
mirrors=
reset_out
run_install "$work/uname-linux-amd64" WEBMUX_CHANNEL=dev >"$work/dev.log" 2>&1 || {
	cat "$work/dev.log"
	fail "dev channel installation failed"
}
grep -q 'webmux-0.10.3-dev.4-linux-amd64 dev' "$work/out/webmux" || fail "dev channel installed the wrong asset"
grep -qF '0.10.3-dev.4' "$work/dev.log" || fail "dev channel did not report the installed version"

reset_out
run_install "$work/uname-darwin-arm64" WEBMUX_VERSION=0.10.3-dev.4 >"$work/dev-pinned.log" 2>&1 || {
	cat "$work/dev-pinned.log"
	fail "installing the current development version failed"
}
grep -q 'webmux-0.10.3-dev.4-darwin-arm64 dev' "$work/out/webmux" || fail "current development version installed the wrong asset"

expect_failure "$work/dev-stale.log" 'the current development version is 0.10.3-dev.4' \
	"$work/uname-linux-amd64" WEBMUX_VERSION=0.10.3-dev.3
expect_failure "$work/dev-release-version.log" 'WEBMUX_VERSION=0.2.0 is a release' \
	"$work/uname-linux-amd64" WEBMUX_CHANNEL=dev WEBMUX_VERSION=0.2.0
expect_failure "$work/stable-dev-version.log" 'cannot be installed with WEBMUX_CHANNEL=stable' \
	"$work/uname-linux-amd64" WEBMUX_CHANNEL=stable WEBMUX_VERSION=0.10.3-dev.4
expect_failure "$work/bad-channel.log" 'invalid WEBMUX_CHANNEL: beta' \
	"$work/uname-linux-amd64" WEBMUX_CHANNEL=beta
expect_failure "$work/bad-version.log" 'invalid WEBMUX_VERSION: 0.10.3-rc.1' \
	"$work/uname-linux-amd64" WEBMUX_VERSION=0.10.3-rc.1

base=$server/dev-mismatch
expect_failure "$work/dev-sig-mismatch.log" 'SHA256SUMS.minisig is for 0.10.3-dev.4, SHA256SUMS lists 0.10.3-dev.3' \
	"$work/uname-linux-amd64" WEBMUX_CHANNEL=dev

base=$server/ambiguous
expect_failure "$work/dev-several.log" 'lists several webmux-<X.Y.Z-dev.N>-linux-amd64' \
	"$work/uname-linux-amd64" WEBMUX_CHANNEL=dev
fake_uname "$work/uname-linux-arm64" Linux aarch64
expect_failure "$work/dev-none.log" 'lists no webmux-<X.Y.Z-dev.N>-linux-arm64' \
	"$work/uname-linux-arm64" WEBMUX_CHANNEL=dev

fake_uname "$work/uname-windows" MINGW64_NT-10.0 x86_64
if run_install "$work/uname-windows" >"$work/windows.log" 2>&1; then
	fail "Windows installation unexpectedly succeeded"
fi
grep -q 'Windows' "$work/windows.log" || fail "Windows failure did not mention the supported download"

fake_uname "$work/uname-freebsd" FreeBSD amd64
if run_install "$work/uname-freebsd" >"$work/freebsd.log" 2>&1; then
	fail "unsupported OS unexpectedly succeeded"
fi
grep -q 'unsupported operating system' "$work/freebsd.log" || fail "unsupported OS error was unclear"

printf 'install.sh: platform, channel, signature, checksum, mirror and error-path checks passed\n'
