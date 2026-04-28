#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
ASSETS_DIR="$ROOT_DIR/assets"
ANDROID_RES_DIR="$ROOT_DIR/android/app/src/main/res"

DEFAULT_SOURCE="$ASSETS_DIR/logo.png"
DEFAULT_ICON_SIZE="736"
DEFAULT_ADAPTIVE_SIZE="640"
DEFAULT_FAVICON_SIZE="184"
DEFAULT_PAD_COLOR="FFFFFF"

usage() {
  cat <<'EOF'
Usage:
  bash update-icon.sh [source.png] [--icon-size N] [--adaptive-size N] [--favicon-size N] [--pad-color HEX] [--skip-android]

Examples:
  bash update-icon.sh
  bash update-icon.sh ./assets/logo.png
  bash update-icon.sh ./assets/new-logo.png --icon-size 700 --adaptive-size 620
  bash update-icon.sh --skip-android

Notes:
  - The source image should be a square PNG for best results.
  - Android mipmap resources are regenerated when the native android directory exists.
EOF
}

require_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

require_integer() {
  case "$2" in
    ''|*[!0-9]*)
      echo "$1 must be a positive integer. Received: $2" >&2
      exit 1
      ;;
  esac

  if [ "$2" -le 0 ]; then
    echo "$1 must be greater than 0. Received: $2" >&2
    exit 1
  fi
}

SOURCE_PATH="$DEFAULT_SOURCE"
ICON_SIZE="$DEFAULT_ICON_SIZE"
ADAPTIVE_SIZE="$DEFAULT_ADAPTIVE_SIZE"
FAVICON_SIZE="$DEFAULT_FAVICON_SIZE"
PAD_COLOR="$DEFAULT_PAD_COLOR"
SKIP_ANDROID="false"

while [ $# -gt 0 ]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    --source)
      if [ $# -lt 2 ]; then
        echo "Missing value for --source" >&2
        exit 1
      fi
      SOURCE_PATH="$2"
      shift 2
      ;;
    --icon-size)
      if [ $# -lt 2 ]; then
        echo "Missing value for --icon-size" >&2
        exit 1
      fi
      ICON_SIZE="$2"
      shift 2
      ;;
    --adaptive-size)
      if [ $# -lt 2 ]; then
        echo "Missing value for --adaptive-size" >&2
        exit 1
      fi
      ADAPTIVE_SIZE="$2"
      shift 2
      ;;
    --favicon-size)
      if [ $# -lt 2 ]; then
        echo "Missing value for --favicon-size" >&2
        exit 1
      fi
      FAVICON_SIZE="$2"
      shift 2
      ;;
    --pad-color)
      if [ $# -lt 2 ]; then
        echo "Missing value for --pad-color" >&2
        exit 1
      fi
      PAD_COLOR="$2"
      shift 2
      ;;
    --skip-android)
      SKIP_ANDROID="true"
      shift
      ;;
    -*)
      echo "Unsupported argument: $1" >&2
      echo >&2
      usage >&2
      exit 1
      ;;
    *)
      if [ "$SOURCE_PATH" != "$DEFAULT_SOURCE" ]; then
        echo "Unexpected argument: $1" >&2
        echo >&2
        usage >&2
        exit 1
      fi
      SOURCE_PATH="$1"
      shift
      ;;
  esac
done

if [ ! -f "$SOURCE_PATH" ]; then
  echo "Source image not found: $SOURCE_PATH" >&2
  exit 1
fi

require_integer "--icon-size" "$ICON_SIZE"
require_integer "--adaptive-size" "$ADAPTIVE_SIZE"
require_integer "--favicon-size" "$FAVICON_SIZE"

case "$PAD_COLOR" in
  [0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f]) ;;
  *)
    echo "--pad-color must be a 6-digit hex color, for example FFFFFF" >&2
    exit 1
    ;;
esac

require_command sips

if [ "$SKIP_ANDROID" != "true" ] && [ -d "$ANDROID_RES_DIR" ]; then
  require_command cwebp
fi

SOURCE_WIDTH="$(sips -g pixelWidth "$SOURCE_PATH" | awk '/pixelWidth/ { print $2 }')"
SOURCE_HEIGHT="$(sips -g pixelHeight "$SOURCE_PATH" | awk '/pixelHeight/ { print $2 }')"

if [ "$SOURCE_WIDTH" != "$SOURCE_HEIGHT" ]; then
  echo "The source image must be square. Current size: ${SOURCE_WIDTH}x${SOURCE_HEIGHT}" >&2
  exit 1
fi

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/quickque-icon.XXXXXX")"
trap 'rm -rf "$TMP_DIR"' EXIT

render_padded_png() {
  local source_path="$1"
  local inner_size="$2"
  local canvas_size="$3"
  local out_path="$4"
  local inner_path="$TMP_DIR/$(basename "$out_path").inner.png"

  sips -z "$inner_size" "$inner_size" "$source_path" --out "$inner_path" >/dev/null
  sips -p "$canvas_size" "$canvas_size" --padColor "$PAD_COLOR" "$inner_path" --out "$out_path" >/dev/null
}

render_webp() {
  local size="$1"
  local source_path="$2"
  local out_path="$3"

  cwebp -quiet -resize "$size" "$size" -q 100 "$source_path" -o "$out_path"
}

ICON_TMP="$TMP_DIR/icon.png"
ADAPTIVE_TMP="$TMP_DIR/adaptive-icon.png"
FAVICON_TMP="$TMP_DIR/favicon.png"

render_padded_png "$SOURCE_PATH" "$ICON_SIZE" "1024" "$ICON_TMP"
render_padded_png "$SOURCE_PATH" "$ADAPTIVE_SIZE" "1024" "$ADAPTIVE_TMP"
render_padded_png "$SOURCE_PATH" "$FAVICON_SIZE" "256" "$FAVICON_TMP"

cp "$ICON_TMP" "$ASSETS_DIR/icon.png"
cp "$ADAPTIVE_TMP" "$ASSETS_DIR/adaptive-icon.png"
cp "$FAVICON_TMP" "$ASSETS_DIR/favicon.png"

if [ "$SKIP_ANDROID" != "true" ] && [ -d "$ANDROID_RES_DIR" ]; then
  while read -r size source_file out_file; do
    render_webp "$size" "$source_file" "$out_file"
  done <<EOF
48 $ICON_TMP $ANDROID_RES_DIR/mipmap-mdpi/ic_launcher.webp
48 $ICON_TMP $ANDROID_RES_DIR/mipmap-mdpi/ic_launcher_round.webp
72 $ICON_TMP $ANDROID_RES_DIR/mipmap-hdpi/ic_launcher.webp
72 $ICON_TMP $ANDROID_RES_DIR/mipmap-hdpi/ic_launcher_round.webp
96 $ICON_TMP $ANDROID_RES_DIR/mipmap-xhdpi/ic_launcher.webp
96 $ICON_TMP $ANDROID_RES_DIR/mipmap-xhdpi/ic_launcher_round.webp
144 $ICON_TMP $ANDROID_RES_DIR/mipmap-xxhdpi/ic_launcher.webp
144 $ICON_TMP $ANDROID_RES_DIR/mipmap-xxhdpi/ic_launcher_round.webp
192 $ICON_TMP $ANDROID_RES_DIR/mipmap-xxxhdpi/ic_launcher.webp
192 $ICON_TMP $ANDROID_RES_DIR/mipmap-xxxhdpi/ic_launcher_round.webp
108 $ADAPTIVE_TMP $ANDROID_RES_DIR/mipmap-mdpi/ic_launcher_foreground.webp
162 $ADAPTIVE_TMP $ANDROID_RES_DIR/mipmap-hdpi/ic_launcher_foreground.webp
216 $ADAPTIVE_TMP $ANDROID_RES_DIR/mipmap-xhdpi/ic_launcher_foreground.webp
324 $ADAPTIVE_TMP $ANDROID_RES_DIR/mipmap-xxhdpi/ic_launcher_foreground.webp
432 $ADAPTIVE_TMP $ANDROID_RES_DIR/mipmap-xxxhdpi/ic_launcher_foreground.webp
EOF
fi

echo "================================"
echo "Knowvia icon update"
echo "================================"
echo "Source: $SOURCE_PATH"
echo "icon.png: ${ICON_SIZE}/1024"
echo "adaptive-icon.png: ${ADAPTIVE_SIZE}/1024"
echo "favicon.png: ${FAVICON_SIZE}/256"
echo "Pad color: #$PAD_COLOR"
if [ "$SKIP_ANDROID" = "true" ]; then
  echo "Android mipmap update: skipped"
elif [ -d "$ANDROID_RES_DIR" ]; then
  echo "Android mipmap update: completed"
else
  echo "Android mipmap update: android directory not found"
fi
echo "================================"
