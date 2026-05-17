#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
ANDROID_DIR="$ROOT_DIR/android"
BUILD_GRADLE="$ANDROID_DIR/app/build.gradle"
DIST_DIR="$ROOT_DIR/dist/android"
DEFAULT_BUILD_TARGET="release"
DEFAULT_VERSION_TYPE="patch"

usage() {
  cat <<'EOF'
Usage:
  bash build_android.sh [debug|release|aab] [options]

Options:
  --type <patch|minor|major>       Version increment type. Defaults to patch.
  --build-type <debug|release|aab> Build target. Positional target is also supported.
  --no-increment                   Build without changing versionName.
  --version-only                   Update versionName without building.
  --clean                          Run Gradle clean before building.
  -h, --help                       Show this help message.

Examples:
  bash build_android.sh
  bash build_android.sh debug
  bash build_android.sh release --clean
  bash build_android.sh aab
  bash build_android.sh --type minor
  bash build_android.sh --build-type debug --no-increment
  bash build_android.sh --type patch --version-only
EOF
}

error() {
  echo "$1" >&2
}

validate_build_target() {
  case "$1" in
    debug|release|aab)
      ;;
    *)
      error "Unsupported build target: $1"
      exit 1
      ;;
  esac
}

validate_version_type() {
  case "$1" in
    patch|minor|major)
      ;;
    *)
      error "Unsupported version type: $1"
      exit 1
      ;;
  esac
}

get_current_version() {
  local version_line
  version_line="$(grep -E 'versionName[[:space:]]+"[^"]+"' "$BUILD_GRADLE" | head -1 || true)"

  if [ -z "$version_line" ]; then
    error "Unable to find versionName in $BUILD_GRADLE"
    exit 1
  fi

  local version
  version="$(printf '%s\n' "$version_line" | sed -E 's/.*versionName[[:space:]]+"([^"]+)".*/\1/')"

  if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    error "Unsupported versionName format: $version"
    error "Expected semantic version format: x.y.z"
    exit 1
  fi

  printf '%s\n' "$version"
}

increment_version() {
  local version="$1"
  local version_type="$2"
  local major minor patch

  IFS='.' read -r major minor patch <<<"$version"

  case "$version_type" in
    major)
      major=$((major + 1))
      minor=0
      patch=0
      ;;
    minor)
      minor=$((minor + 1))
      patch=0
      ;;
    patch)
      patch=$((patch + 1))
      ;;
  esac

  printf '%s.%s.%s\n' "$major" "$minor" "$patch"
}

update_version_in_gradle() {
  local old_version="$1"
  local new_version="$2"
  local escaped_old_version="${old_version//./\\.}"

  if [[ "${OSTYPE:-}" == darwin* ]]; then
    sed -i '' "s/versionName \"$escaped_old_version\"/versionName \"$new_version\"/g" "$BUILD_GRADLE"
  else
    sed -i "s/versionName \"$escaped_old_version\"/versionName \"$new_version\"/g" "$BUILD_GRADLE"
  fi
}

BUILD_TARGET="$DEFAULT_BUILD_TARGET"
VERSION_TYPE="$DEFAULT_VERSION_TYPE"
SHOULD_INCREMENT="true"
SHOULD_BUILD="true"
SHOULD_CLEAN="false"

while [ "$#" -gt 0 ]; do
  case "$1" in
    debug|release|aab)
      BUILD_TARGET="$1"
      shift
      ;;
    --build-type)
      if [ -z "${2:-}" ]; then
        error "--build-type requires a value: debug, release, or aab"
        exit 1
      fi
      validate_build_target "$2"
      BUILD_TARGET="$2"
      shift 2
      ;;
    --type)
      if [ -z "${2:-}" ]; then
        error "--type requires a value: patch, minor, or major"
        exit 1
      fi
      validate_version_type "$2"
      VERSION_TYPE="$2"
      shift 2
      ;;
    --no-increment)
      SHOULD_INCREMENT="false"
      shift
      ;;
    --version-only)
      SHOULD_BUILD="false"
      shift
      ;;
    --clean)
      SHOULD_CLEAN="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unsupported argument: $1" >&2
      echo >&2
      usage >&2
      exit 1
      ;;
  esac
done

validate_build_target "$BUILD_TARGET"
validate_version_type "$VERSION_TYPE"

if [ ! -d "$ANDROID_DIR" ]; then
  echo "Missing android directory: $ANDROID_DIR" >&2
  exit 1
fi

if [ ! -f "$BUILD_GRADLE" ]; then
  echo "Missing Android build file: $BUILD_GRADLE" >&2
  exit 1
fi

if [ ! -f "$ANDROID_DIR/gradlew" ]; then
  echo "Missing Gradle wrapper: $ANDROID_DIR/gradlew" >&2
  exit 1
fi

CURRENT_VERSION="$(get_current_version)"
APP_VERSION="$CURRENT_VERSION"

if [ "$SHOULD_INCREMENT" = "true" ]; then
  APP_VERSION="$(increment_version "$CURRENT_VERSION" "$VERSION_TYPE")"
  update_version_in_gradle "$CURRENT_VERSION" "$APP_VERSION"
  echo "Version updated: $CURRENT_VERSION -> $APP_VERSION ($VERSION_TYPE)"
else
  echo "Version unchanged: $APP_VERSION"
fi

if [ "$SHOULD_BUILD" = "false" ]; then
  echo "Version-only mode enabled. Skipping Android build."
  exit 0
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
