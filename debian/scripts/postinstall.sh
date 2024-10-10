#!/bin/sh

cleanInstall() {
    adduser --system --disabled-login --group --disabled-password checkrr
    systemctl daemon-reload
    systemctl unmask checkrr
    systemctl preset checkrr
    systemctl enable checkrr
    systemctl restart checkrr
}

upgrade() {
    systemctl restart checkrr
}

# Step 2, check if this is a clean install or an upgrade
action="$1"
if  [ "$1" = "configure" ] && [ -z "$2" ]; then
  # Alpine linux does not pass args, and deb passes $1=configure
  action="install"
elif [ "$1" = "configure" ] && [ -n "$2" ]; then
    # deb passes $1=configure $2=<current version>
    action="upgrade"
fi

case "$action" in
  "1" | "install")
    cleanInstall
    ;;
  "2" | "upgrade")
    upgrade
    ;;
  *)
    cleanInstall
    ;;
esac