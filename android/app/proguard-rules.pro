# EXRA Node Proguard Rules

# BouncyCastle - Critical for JCE providers and cryptographic algorithms
-keep class org.bouncycastle.** { *; }
-dontwarn org.bouncycastle.**

# Substrate SDK - Critical for Sr25519 and Ed25519 signing (Native bridges)
-keep class io.novafoundation.nova.** { *; }
-keep class io.novasamatech.substrate_sdk_android.** { *; }
-dontwarn io.novafoundation.nova.**
-dontwarn io.novasamatech.substrate_sdk_android.**

# Kotlin Coroutines - Prevent stripping of suspending functions and dispatchers
-keep class kotlinx.coroutines.** { *; }
-dontwarn kotlinx.coroutines.**

# OkHttp/Okio - Prevent issues with network and WebSocket framing
-keep class okhttp3.** { *; }
-keep class okio.** { *; }
-dontwarn okhttp3.**
-dontwarn okio.**

# Compose - Maintain Material 3 and Navigation stability after minification
-keep class androidx.compose.** { *; }
-dontwarn androidx.compose.**

# Security Crypto
-keep class androidx.security.crypto.** { *; }
-dontwarn androidx.security.crypto.**
