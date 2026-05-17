# Android Packaging Version Bump Design

## Goal

Enhance the existing Android packaging script in `app/build_android.sh` so it behaves like the Oasyce frontend Android build script while preserving this project's Expo/React Native build flow.

## Scope

- Keep `app/build_android.sh` as the single Android packaging entrypoint.
- Support `debug`, `release`, and `aab` build targets.
- Add version bump flags: `--type patch|minor|major`, `--no-increment`, and `--version-only`.
- Update `app/android/app/build.gradle` `versionName` before packaging by default.
- Preserve Node 20, dependency, Gradle wrapper, build target, artifact copy, and `--clean` checks already in the script.
- Add npm shortcuts for patch, minor, major, debug, release, and AAB builds.

## Behavior

Default invocation increments the patch version and builds a release APK:

```bash
npm run build:android
```

Callers can opt out of version changes:

```bash
npm run build:android:release -- --no-increment
```

Callers can update the version without building:

```bash
bash app/build_android.sh --type minor --version-only
```

## Verification

- `bash app/build_android.sh --help` shows the new flags.
- `bash app/build_android.sh --type patch --version-only` updates `versionName`.
- `bash app/build_android.sh --type patch --version-only` can be reverted cleanly after verification so the workspace does not keep an accidental version bump.
