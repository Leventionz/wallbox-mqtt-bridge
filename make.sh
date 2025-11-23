set -x

VERSION="${BRIDGE_VERSION:-}"
if [ -z "$VERSION" ]; then
    VERSION="$(git describe --tags --dirty --always 2>/dev/null || echo dev)"
fi

LDFLAGS="-s -w -X=wallbox-mqtt-bridge/app.buildVersion=${VERSION}"

CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -ldflags="$LDFLAGS" -o bridge-armhf .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="$LDFLAGS" -o bridge-arm64 .
