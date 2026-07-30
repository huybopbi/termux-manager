#!/data/data/com.termux/files/usr/bin/bash
set -e

REPO="huybopbi/termux-manager"
BIN="$PREFIX/bin/manager"
VERSION_URL="https://api.github.com/repos/$REPO/releases/latest"

echo "▶ termux-manager installer"
echo ""

# Detect arch
ARCH=$(uname -m)
case $ARCH in
  aarch64|arm64) GOARCH="arm64" ;;
  armv7l|armv8l) GOARCH="arm"   ;;
  x86_64)        GOARCH="amd64" ;;
  *)
    echo "✗ Unsupported architecture: $ARCH"
    exit 1
  ;;
esac

echo "▷ Architecture : $ARCH ($GOARCH)"

# Install wget if missing
if ! command -v wget &>/dev/null; then
  echo "▷ Installing wget..."
  pkg install -y wget
fi

# Try to fetch latest release tag
if command -v curl &>/dev/null; then
  TAG=$(curl -sf "$VERSION_URL" | grep '"tag_name"' | cut -d'"' -f4)
elif command -v wget &>/dev/null; then
  TAG=$(wget -qO- "$VERSION_URL" | grep '"tag_name"' | cut -d'"' -f4)
fi

if [ -z "$TAG" ]; then
  echo "▷ Could not fetch latest release, using fallback binary name"
  BINARY_NAME="manager-android-${GOARCH}"
else
  BINARY_NAME="manager-android-${GOARCH}"
  echo "▷ Latest release : $TAG"
fi

DOWNLOAD_URL="https://github.com/$REPO/releases/download/$TAG/$BINARY_NAME"

echo "▷ Downloading    : $BINARY_NAME"
wget -q --show-progress -O "$BIN" "$DOWNLOAD_URL"
chmod +x "$BIN"

echo ""
echo "✓ Installed to $BIN"
echo ""
echo "Usage:"
echo "  manager              # start on default port 9876"
echo "  manager -port 8080   # custom port"
echo "  manager -root /sdcard # custom root directory"
echo "  manager -hidden      # show hidden files"
echo "  manager -help        # all flags"
echo ""
echo "Run 'manager' to start!"
