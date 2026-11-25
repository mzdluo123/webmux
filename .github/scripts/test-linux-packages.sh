#!/bin/sh
# Install, upgrade, inspect and remove the deb and rpm packages for this
# machine's architecture in clean Debian and Fedora containers.
#
# Usage: test-linux-packages.sh <directory with the packages> <version>
# DEBIAN_IMAGE / FEDORA_IMAGE override the images (for example a registry
# mirror such as public.ecr.aws/docker/library/debian:stable-slim).
set -eu

dir=$(CDPATH= cd -- "$1" && pwd)
version=$2
debian_image=${DEBIAN_IMAGE:-debian:stable-slim}
fedora_image=${FEDORA_IMAGE:-fedora:latest}

case $(uname -m) in
x86_64 | amd64) deb_arch=amd64 rpm_arch=x86_64 ;;
aarch64 | arm64) deb_arch=arm64 rpm_arch=aarch64 ;;
*)
	printf 'test-linux-packages: unsupported architecture %s\n' "$(uname -m)" >&2
	exit 1
	;;
esac
deb=webmux_${version}-1_${deb_arch}.deb
rpm=webmux-${version}-1.${rpm_arch}.rpm
case $version in
*-dev.*) server=webmux-$version-linux-$deb_arch ;;
*) server=webmux-linux-$deb_arch ;;
esac
for file in "$deb" "$rpm" "$server"; do
	[ -f "$dir/$file" ] || {
		printf 'test-linux-packages: missing %s/%s\n' "$dir" "$file" >&2
		exit 1
	}
done
server_sha=$(sha256sum "$dir/$server" | cut -d' ' -f1)
repo=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
license_sha=$(awk '$1 == "license" { print $2 }' "$repo/internal/frphelper/pins.txt")

# A development version X.Y.Z-dev.N is packaged as X.Y.Z~dev.N, which dpkg
# and rpm sort before X.Y.Z; file names keep the "-".
case $version in
*-*) package_version="${version%%-*}~${version#*-}-1" ;;
*) package_version="$version-1" ;;
esac

# Shared checks after installation: the binary, units and notices, their modes, the units'
# commands and the version the binary reports.
checks='
	test "$(stat -c %a /usr/bin/webmux)" = 755
	test "$(sha256sum /usr/bin/webmux | cut -d" " -f1)" = "$SERVER_SHA"
	test "$(stat -c %a /usr/lib/systemd/user/webmux.service)" = 644
	test "$(stat -c %a /usr/lib/systemd/system/webmux-update.service)" = 644
	test "$(stat -c %a /usr/lib/systemd/system/webmux-update.timer)" = 644
	test "$(stat -c %a /usr/share/licenses/webmux/OFL-NotoSansCJK.txt)" = 644
	test "$(stat -c %a /usr/share/licenses/webmux/Snapcompact-Go-NOTICES.txt)" = 644
	test "$(stat -c %a /usr/share/licenses/webmux/frpc.LICENSE)" = 644
	test "$(sha256sum /usr/share/licenses/webmux/frpc.LICENSE | cut -d" " -f1)" = "$LICENSE_SHA"
	grep -q "SIL OPEN FONT LICENSE Version 1.1" /usr/share/licenses/webmux/OFL-NotoSansCJK.txt
	grep -q "Copyright (c) 2014-2021 Adobe" /usr/share/licenses/webmux/OFL-NotoSansCJK.txt
	grep -q "MIT License" /usr/share/licenses/webmux/Snapcompact-Go-NOTICES.txt
	grep -qx "ExecStart=/usr/bin/webmux serve" /usr/lib/systemd/user/webmux.service
	grep -qx "ExecStart=/usr/bin/webmux update apply" /usr/lib/systemd/system/webmux-update.service
	test "$(/usr/bin/webmux version)" = "webmux $VERSION"
'
removed='
	test ! -e /usr/share/licenses/webmux/OFL-NotoSansCJK.txt
	test ! -e /usr/share/licenses/webmux/Snapcompact-Go-NOTICES.txt
	test ! -e /usr/share/licenses/webmux/frpc.LICENSE
	test ! -e /usr/bin/webmux
	test ! -e /usr/lib/systemd/user/webmux.service
	test ! -e /usr/lib/systemd/system/webmux-update.service
	test ! -e /usr/lib/systemd/system/webmux-update.timer
'
# A recording systemctl and a /run/systemd/system marker stand in for a
# booted systemd, so the maintainer scripts take their systemd paths.
fake_systemd='
	mkdir -p /run/systemd/system
	printf "#!/bin/sh\necho \"\$*\" >> /tmp/systemctl.log\n" > /usr/bin/systemctl
	chmod 755 /usr/bin/systemctl
	: > /tmp/systemctl.log
'
# First install enables the timer and leaves a marker; an upgrade with the
# marker only reloads (an admin's disable survives); an upgrade without it,
# as from a release that predates the timer, enables it; removal disables
# it and drops the marker. webmux.service is never named.
timer_calls='
	marker=/var/lib/webmux/auto-update-enabled
	log() { cat /tmp/systemctl.log; : > /tmp/systemctl.log; }
	never_user_service() { ! grep -q "webmux.service" /tmp/systemctl.log; }
'

docker run --rm -e VERSION="$version" -e SERVER_SHA="$server_sha" -e LICENSE_SHA="$license_sha" -v "$dir:/pkgs:ro" "$debian_image" sh -euc "
	test -z \"\$(dpkg-deb -f /pkgs/$deb Depends)\"
	test \"\$(dpkg-deb -f /pkgs/$deb Version)\" = '$package_version'
	test \"\$(dpkg-deb -c /pkgs/$deb | awk '\$1 !~ /^d/ {print \$6}' | LC_ALL=C sort | tr '\n' ' ')\" = './usr/bin/webmux ./usr/lib/systemd/system/webmux-update.service ./usr/lib/systemd/system/webmux-update.timer ./usr/lib/systemd/user/webmux.service ./usr/share/licenses/webmux/OFL-NotoSansCJK.txt ./usr/share/licenses/webmux/Snapcompact-Go-NOTICES.txt ./usr/share/licenses/webmux/frpc.LICENSE '

	# Without systemd: installs and removes quietly, enabling nothing.
	dpkg -i /pkgs/$deb >/dev/null
	$checks
	dpkg -P webmux >/dev/null
	$removed

	$fake_systemd
	$timer_calls
	dpkg -i /pkgs/$deb >/dev/null
	never_user_service
	log | grep -qx 'enable --now webmux-update.timer'
	test -e \$marker
	dpkg -i /pkgs/$deb >/dev/null
	never_user_service
	if log | grep -q enable; then exit 1; fi
	rm \$marker
	dpkg -i /pkgs/$deb >/dev/null
	log | grep -qx 'enable --now webmux-update.timer'
	dpkg -r webmux >/dev/null
	never_user_service
	log | grep -qx 'disable --now webmux-update.timer'
	test ! -e \$marker
	$removed
" || {
	printf 'test-linux-packages: %s failed its checks on Debian\n' "$deb" >&2
	exit 1
}
printf 'ok - %s installs, upgrades, runs and removes on Debian\n' "$deb"

docker run --rm -e VERSION="$version" -e SERVER_SHA="$server_sha" -e LICENSE_SHA="$license_sha" -v "$dir:/pkgs:ro" "$fedora_image" sh -euc "
	test \"\$(rpm -qpl /pkgs/$rpm | LC_ALL=C sort | tr '\n' ' ')\" = '/usr/bin/webmux /usr/lib/systemd/system/webmux-update.service /usr/lib/systemd/system/webmux-update.timer /usr/lib/systemd/user/webmux.service /usr/share/licenses/webmux/OFL-NotoSansCJK.txt /usr/share/licenses/webmux/Snapcompact-Go-NOTICES.txt /usr/share/licenses/webmux/frpc.LICENSE '
	test -z \"\$(rpm -qpR /pkgs/$rpm | grep -v '^rpmlib(' | grep -v '^/bin/sh' || true)\"
	test \"\$(rpm -qp --qf '%{VERSION}-%{RELEASE}' /pkgs/$rpm)\" = '$package_version'

	rpm -i --nosignature /pkgs/$rpm
	$checks
	rpm -e webmux
	$removed

	$fake_systemd
	$timer_calls
	rpm -i --nosignature /pkgs/$rpm
	never_user_service
	log | grep -qx 'enable --now webmux-update.timer'
	test -e \$marker
	rpm -U --nosignature --replacepkgs /pkgs/$rpm
	never_user_service
	if log | grep -q enable; then exit 1; fi
	rm \$marker
	rpm -U --nosignature --replacepkgs /pkgs/$rpm
	log | grep -qx 'enable --now webmux-update.timer'
	rpm -e webmux
	never_user_service
	log | grep -qx 'disable --now webmux-update.timer'
	test ! -e \$marker
	$removed
" || {
	printf 'test-linux-packages: %s failed its checks on Fedora\n' "$rpm" >&2
	exit 1
}
printf 'ok - %s installs, upgrades, runs and removes on Fedora\n' "$rpm"
