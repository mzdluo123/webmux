#!/bin/sh
# deb postinst / rpm %post for the WebMux package. A first install enables
# the automatic update timer; an upgrade only reloads unit definitions, so an
# administrator's `systemctl disable` survives. Never touches webmux.service:
# restarting it would end every terminal session.
#
# Arguments: deb "configure [<old version>]"; rpm "1" (install) or "2" (upgrade).
set -e

first=0
case "${1:-}" in
configure) [ -n "${2:-}" ] || first=1 ;;
1) first=1 ;;
esac

# Containers and chroots without a running systemd install the files only.
if [ -d /run/systemd/system ]; then
	systemctl daemon-reload >/dev/null 2>&1 || true
	if [ "$first" = 1 ]; then
		systemctl enable --now webmux-update.timer >/dev/null 2>&1 || true
	fi
fi
exit 0
