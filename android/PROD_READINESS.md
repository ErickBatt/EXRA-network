# Google Play Production Readiness Guide (EXRA)

This guide provides the necessary justifications and configurations needed for a successful Google Play Store submission.

## 1. Foreground Service Declaration (API 34+)

When submitting to Google Play, you will be asked to justify the use of `FOREGROUND_SERVICE_DATA_SYNC`.

**Service Type:** `dataSync`
**Justification for Google Play Console:**
> "The EXRA Node application acts as a decentralized network participant (DePIN). The foreground service is essential to maintain a continuous, low-latency connection with the network oracle. It performs real-time synchronization of network traffic throughput and Proof-of-Performance (PoP) metrics. Without this service, the application cannot accurately report work done by the node, resulting in loss of cryptographically secured rewards for the user."

---

## 2. Infrastructure Setup (Action Required)

### Firebase google-services.json
Before building the production APK:
1. Go to [Firebase Console](https://console.firebase.google.com/).
2. Create/Select your project.
3. Add an Android app with package name `io.exra.node`.
4. Download `google-services.json`.
5. Place it in: `android/app/google-services.json`.

---

## 3. Production Secrets (Environment Variables)

The build script is now protected. To build the production variant, you **must** set the environment variable:
- `NODE_SECRET`: Your production node access secret.

**Build Command:**
```bash
# Example for CI or local terminal
$env:NODE_SECRET="your_real_secret_here"
./gradlew assembleRelease
```

## Firebase Activation (Post-Build)
To enable production observability (Crashlytics & Analytics), follow these steps once you have the `google-services.json` file:

1.  **Place File**: Copy `google-services.json` to `android/app/`.
2.  **Enable Plugins**: In `android/app/build.gradle`, uncomment the following lines in the `plugins` block:
    ```gradle
    // id 'com.google.gms.google-services'
    // id 'com.google.firebase.crashlytics'
    ```
3.  **Enable Code**: In `ExraApplication.kt`, remove the comment marks `/*` and `*/` surrounding the Firebase initialization block.
4.  **Rebuild**: Run `./gradlew assembleRelease`.

---

## 4. Google Play Data Safety Questionnaire

When filling out the "Data Safety" section in Play Console, you must declare that the app collects the following data:

| Data Type | Purpose | Encryption |
| :--- | :--- | :--- |
| **Device IDs** | Node identification & Sybil protection | Encrypted in transit (WSS) |
| **Approximate Location** | Regional GearScore calculation | Encrypted in transit (WSS) |
| **Network Performance** | Proof-of-Performance (Traffic synchronization) | Encrypted in transit (WSS) |

**Encryption Statement:** 
Answer "Yes" to "Is all of the user data collected by your app encrypted in transit?". Explain that all telemetry and node-to-server communication uses industry-standard WSS (WebSocket over TLS).

---

## 5. Privacy Policy Requirements
Since the node shares internet traffic (as a proxy), ensure your Privacy Policy explicitly states:
- That the app utilizes a portion of the user's internet bandwidth.
- That only metadata/traffic volume is reported to the server for PoP verification.
- No personal user data (PII) is captured during network proxying.

---
**EXRA Node is ready for Mainnet Deployment.** 🚀🟢🏆
