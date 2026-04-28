#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
ANDROID_DIR="$ROOT_DIR/android"
DIST_DIR="$ROOT_DIR/dist/android"
DEFAULT_BUILD_TARGET="release"

usage() {
  cat <<'EOF'
Usage:
  bash build_android.sh [debug|release|aab] [--clean]

Examples:
  bash build_android.sh
  bash build_android.sh debug
  bash build_android.sh release --clean
  bash build_android.sh aab
EOF
}

BUILD_TARGET="$DEFAULT_BUILD_TARGET"
SHOULD_CLEAN="false"

for arg in "$@"; do
  case "$arg" in
    debug|release|aab)
      BUILD_TARGET="$arg"
      ;;
    --clean)
      SHOULD_CLEAN="true"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unsupported argument: $arg" >&2
      echo >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [ ! -d "$ANDROID_DIR" ]; then
  echo "Missing android directory: $ANDROID_DIR" >&2
  exit 1
fi

if [ ! -f "$ANDROID_DIR/gradlew" ]; then
  echo "Missing Gradle wrapper: $ANDROID_DIR/gradlew" >&2
  exit 1
fi

if [ -f "$ROOT_DIR/.nvmrc" ]; then
  export NVM_DIR="${NVM_DIR:-$HOME/.nvm}"

  if [ -s "$NVM_DIR/nvm.sh" ]; then
    # Align the shell Node version with the app's pinned runtime.
    # shellcheck disable=SC1090
    . "$NVM_DIR/nvm.sh"
    nvm use >/dev/null
  fi
fi

if ! command -v node >/dev/null 2>&1; then
  echo "Node.js is required to build the Android app." >&2
  exit 1
fi

NODE_MAJOR="$(node -p "process.versions.node.split('.')[0]")"
if [ "$NODE_MAJOR" != "20" ]; then
  echo "Node 20.x is required. Current version: $(node -v)" >&2
  exit 1
fi

if [ ! -d "$ROOT_DIR/node_modules" ]; then
  echo "Dependencies are missing. Run 'npm install' in $ROOT_DIR first." >&2
  exit 1
fi

APP_VERSION="$(node -p "const app = require('./app.json'); app.expo?.version || require('./package.json').version")"

GRADLE_TASK="assembleRelease"
OUTPUT_PATH="$ANDROID_DIR/app/build/outputs/apk/release/app-release.apk"
DIST_FILE="$DIST_DIR/quickque-agent-v${APP_VERSION}-release.apk"

case "$BUILD_TARGET" in
  debug)
    GRADLE_TASK="assembleDebug"
    OUTPUT_PATH="$ANDROID_DIR/app/build/outputs/apk/debug/app-debug.apk"
    DIST_FILE="$DIST_DIR/quickque-agent-v${APP_VERSION}-debug.apk"
    ;;
  release)
    GRADLE_TASK="assembleRelease"
    OUTPUT_PATH="$ANDROID_DIR/app/build/outputs/apk/release/app-release.apk"
    DIST_FILE="$DIST_DIR/quickque-agent-v${APP_VERSION}-release.apk"
    ;;
  aab)
    GRADLE_TASK="bundleRelease"
    OUTPUT_PATH="$ANDROID_DIR/app/build/outputs/bundle/release/app-release.aab"
    DIST_FILE="$DIST_DIR/quickque-agent-v${APP_VERSION}-release.aab"
    ;;
esac

if [ -z "${NODE_ENV:-}" ]; then
  case "$BUILD_TARGET" in
    debug)
      export NODE_ENV="development"
      ;;
    release|aab)
      export NODE_ENV="production"
      ;;
  esac
fi

echo "================================"
echo "Knowvia Android build"
echo "================================"
echo "Target: $BUILD_TARGET"
echo "Version: $APP_VERSION"
echo "Node: $(node -v)"
echo "NODE_ENV: $NODE_ENV"
printf '\n'

cd "$ANDROID_DIR"

if [ "$SHOULD_CLEAN" = "true" ]; then
  echo "Cleaning previous Android build artifacts..."
  ./gradlew clean
  printf '\n'
fi

echo "Running Gradle task: $GRADLE_TASK"
./gradlew "$GRADLE_TASK" -x lintVitalAnalyzeRelease -x lintVitalRelease

if [ ! -f "$OUTPUT_PATH" ]; then
  echo "Build finished but no artifact was found at:" >&2
  echo "  $OUTPUT_PATH" >&2
  exit 1
fi

mkdir -p "$DIST_DIR"
cp "$OUTPUT_PATH" "$DIST_FILE"

printf '\n'
echo "Build succeeded."
echo "Gradle artifact: $OUTPUT_PATH"
echo "Copied artifact: $DIST_FILE"
echo "Artifact size: $(du -h "$DIST_FILE" | cut -f1)"
echo "================================"
