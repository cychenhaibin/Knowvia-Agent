# App Dev Workflow

Android debug builds in this project run through the Expo development client, so UI changes come from the Metro bundle at runtime, not from the APK itself.

Use this flow when you want the device to reliably load the latest code:

1. Start Metro with a clean cache:

```bash
npm run dev:android
```

2. Install or rebuild the debug app when needed:

```bash
npm run android
```

3. After JS-only changes, just reconnect and relaunch the app:

```bash
npm run android:reload
```

Release builds are different: the JS bundle is packaged into the app during build, so they always reflect the code that existed at build time.
