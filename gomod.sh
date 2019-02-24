#!/bin/sh -e

exe() {
    echo '$' $@
    $@
}

# https://askubuntu.com/a/22257
confirm() {
    echo -n "Are you sure you want to run '$*' in $PWD? [N/y] "
    read -N 1 REPLY
    echo
    if test "$REPLY" = "y" -o "$REPLY" = "Y"; then
        exe "$@"
    else
        echo Exiting.
        exit 1
    fi
}

exe cd "${0%/*}"

if [ ! -f go.mod ]; then
    confirm rm -rf Gopkg.toml Gopkg.lock vendor
    exe go mod init github.com/jakebailey/ua
fi

exe go get -u github.com/docker/{docker,distribution,cli}@master
exe go mod edit -replace github.com/satori/go.uuid=github.com/gofrs/uuid@v2.1.0
exe go get -u ./...
exe go mod tidy
exe go mod vendor