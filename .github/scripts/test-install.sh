#!/bin/sh
# Exercise install.sh against a local release fixture. Runs on Linux CI only;
# macOS uses shasum instead of sha256sum, which the fixture does not provide.
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

sh -n "$repo/install.sh" || fail "install.sh has a syntax error"
sh -n "$0" || fail "test-install.sh has a syntax error"

www=$work/www
mkdir -p "$www/releases/latest/download"
for os in linux darwin; do
	for arch in amd64 arm64; do
		asset=webmux-$os-$arch
		printf '#!/bin/sh\n# %s latest\nprintf "webmux 0.2.0\\n"\n' "$asset" > "$www/releases/latest/download/$asset"
		chmod 755 "$www/releases/latest/download/$asset"
	done
done
(cd "$www/releases/latest/download" && sha256sum webmux-* > SHA256SUMS)

mkdir -p "$www/releases/download/v0.2.0"
for os in linux darwin; do
	for arch in amd64 arm64; do
		asset=webmux-$os-$arch
		printf '#!/bin/sh\n# %s pinned\nprintf "webmux 0.2.0\\n"\n' "$asset" > "$www/releases/download/v0.2.0/$asset"
		chmod 755 "$www/releases/download/v0.2.0/$asset"
	done
done
(cd "$www/releases/download/v0.2.0" && sha256sum webmux-* > SHA256SUMS)

mkdir -p "$www/bad/releases/latest/download"
printf 'bad binary\n' > "$www/bad/releases/latest/download/webmux-linux-amd64"
printf '%064d  webmux-linux-amd64\n' 0 > "$www/bad/releases/latest/download/SHA256SUMS"

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
		WEBMUX_BASE_URL="$base" WEBMUX_MIRRORS="$mirrors" "$@" \
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

# A pinned digest must work even when the source has no SHA256SUMS.
mkdir -p "$www/nosums/releases/latest/download"
cp "$www/releases/latest/download/webmux-linux-amd64" "$www/nosums/releases/latest/download/"
pinned_sum=$(sha256sum "$www/nosums/releases/latest/download/webmux-linux-amd64" | awk '{print $1}')
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

# The first source 404s; the gh-proxy style mirror prefix must supply the asset.
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

printf 'install.sh: platform, checksum, mirror and error-path checks passed\n'
