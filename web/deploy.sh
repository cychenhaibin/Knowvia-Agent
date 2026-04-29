#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel)"

TARGET="${DEPLOY_TARGET:-cloudflare}"
REMOTE="${DEPLOY_REMOTE:-origin}"
BRANCH="${DEPLOY_BRANCH:-main}"
PROJECT_NAME="${PROJECT_NAME:-Knowvia-Agent}"
CLOUDFLARE_PROJECT="${CLOUDFLARE_PROJECT:-knowvia}"
CLOUDFLARE_DOMAIN="${CLOUDFLARE_DOMAIN:-knowvia.camila.qzz.io}"
COMMIT_MESSAGE="${DEPLOY_COMMIT_MESSAGE:-deploy web - $(date '+%Y-%m-%d %H:%M:%S')}"

cd "$SCRIPT_DIR"

if ! command -v npm >/dev/null 2>&1; then
  echo "[deploy] npm is required but was not found."
  exit 1
fi

if [ ! -d "$SCRIPT_DIR/node_modules" ]; then
  echo "[deploy] node_modules not found. Installing dependencies with npm ci..."
  npm ci
fi

case "$TARGET" in
  github)
    BASE_PATH="${BASE_PATH:-/${PROJECT_NAME}/}"
    SITE_URL="${DEPLOY_URL:-https://cychenhaibin.github.io/${PROJECT_NAME}/}"
    ;;
  cloudflare)
    BASE_PATH="${BASE_PATH:-/}"
    SITE_URL="${DEPLOY_URL:-https://${CLOUDFLARE_DOMAIN}/}"
    ;;
  *)
    echo "[deploy] Unknown DEPLOY_TARGET: $TARGET"
    echo "[deploy] Supported targets: github, cloudflare"
    exit 1
    ;;
esac

echo "[deploy] Target: $TARGET"
echo "[deploy] Web directory: $SCRIPT_DIR"
echo "[deploy] Base path: $BASE_PATH"

WEB_STATUS="$(git -C "$REPO_ROOT" status --porcelain -- "$SCRIPT_DIR")"
if [ -n "$WEB_STATUS" ]; then
  if [ "${AUTO_COMMIT:-0}" = "1" ]; then
    echo "[deploy] Committing web changes..."
    git -C "$REPO_ROOT" add "$SCRIPT_DIR"
    git -C "$REPO_ROOT" commit -m "$COMMIT_MESSAGE"
  else
    echo "[deploy] Uncommitted changes detected in web/. Commit them first, or run with AUTO_COMMIT=1."
    git -C "$REPO_ROOT" status --short -- "$SCRIPT_DIR"
    exit 1
  fi
fi

echo "[deploy] Building..."
npm run build -- --base "$BASE_PATH"
echo "[deploy] Build completed."

if [ "$TARGET" = "github" ]; then
  echo "[deploy] Pushing $BRANCH to $REMOTE..."
  git -C "$REPO_ROOT" push "$REMOTE" "$BRANCH"
else
  if ! command -v npx >/dev/null 2>&1; then
    echo "[deploy] npx is required for Cloudflare Pages deploy."
    exit 1
  fi

  echo "[deploy] Deploying dist/ to Cloudflare Pages project: $CLOUDFLARE_PROJECT"
  npx wrangler pages deploy dist --project-name "$CLOUDFLARE_PROJECT"
fi

echo "[deploy] Done."
echo "[deploy] Site: $SITE_URL"
