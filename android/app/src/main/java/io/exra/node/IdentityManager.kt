package io.exra.node

import android.content.Context
import android.util.Log
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKeys
import dev.sublab.sr25519.KeyPair
import dev.sublab.sr25519.PublicKey
import dev.sublab.sr25519.SecretKey
import dev.sublab.sr25519.Signature
import org.bouncycastle.util.encoders.Hex
import java.security.MessageDigest
import java.security.SecureRandom
import android.os.Build
import android.provider.Settings
import java.io.File

class IdentityManager(private val context: Context) {
    private val TAG = "IdentityManager"
    private val PREFS_NAME = "ExraIdentitySecure"
    private val KEY_PRIV = "priv_key"
    private val KEY_PUB = "pub_key"
    private val KEY_NONCE = "key_nonce"

    fun isEmulator(): Boolean {
        val basicCheck = (Build.BRAND.startsWith("generic") && Build.DEVICE.startsWith("generic"))
                || Build.FINGERPRINT.startsWith("generic")
                || Build.FINGERPRINT.startsWith("unknown")
                || Build.HARDWARE.contains("goldfish")
                || Build.HARDWARE.contains("ranchu")
                || Build.MODEL.contains("google_sdk")
                || Build.MODEL.contains("Emulator")
                || Build.MODEL.contains("Android SDK built for x86")
                || Build.MANUFACTURER.contains("Genymotion")
                || Build.PRODUCT.contains("sdk_google")
                || Build.PRODUCT.contains("google_sdk")
                || Build.PRODUCT.contains("sdk")
                || Build.PRODUCT.contains("sdk_x86")
                || Build.PRODUCT.contains("vbox86p")
                || Build.PRODUCT.contains("emulator")
                || Build.PRODUCT.contains("simulator")

        return basicCheck || checkFilesystemProbes()
    }

    private fun checkFilesystemProbes(): Boolean {
        val probes = arrayOf(
            "/dev/qemu_pipe",
            "/system/lib/libc_malloc_debug_qemu.so",
            "/sys/qemu_trace",
            "/proc/tty/drivers"
        )
        for (p in probes) {
            if (File(p).exists()) {
                if (p == "/proc/tty/drivers") {
                    try {
                        if (File(p).readText().contains("goldfish")) return true
                    } catch (e: Exception) { /* ignore */ }
                } else {
                    return true
                }
            }
        }
        return false
    }

    fun getHardwareFingerprint(): String {
        val androidId = Settings.Secure.getString(context.contentResolver, Settings.Secure.ANDROID_ID) ?: "no_id"
        val buildInfo = "${androidId}${Build.BOARD}${Build.BRAND}${Build.DEVICE}${Build.HARDWARE}${Build.MANUFACTURER}${Build.MODEL}${Build.PRODUCT}"
        return try {
            val md = MessageDigest.getInstance("SHA-256")
            val digest = md.digest(buildInfo.toByteArray())
            Hex.toHexString(digest)
        } catch (e: Exception) {
            "unknown_hw_hash"
        }
    }

    private var keyPair: KeyPair? = null

    init {
        loadOrGenerateKeys()
    }

    private fun getEncryptedPrefs(): android.content.SharedPreferences {
        val masterKeyAlias = MasterKeys.getOrCreate(MasterKeys.AES256_GCM_SPEC)
        return EncryptedSharedPreferences.create(
            PREFS_NAME,
            masterKeyAlias,
            context,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
        )
    }

    private fun loadOrGenerateKeys() {
        try {
            val prefs = getEncryptedPrefs()
            val privHex = prefs.getString(KEY_PRIV, null)
            val pubHex = prefs.getString(KEY_PUB, null)

            if (privHex == null || pubHex == null) {
                Log.i(TAG, "No keys found in storage, generating new ones...")
                generateKeys()
            } else {
                val privateKeyBytes = Hex.decode(privHex)
                val publicKeyBytes = Hex.decode(pubHex)
                
                Log.d(TAG, "Loading keys: Priv size=${privateKeyBytes.size}, Pub size=${publicKeyBytes.size}")

                try {
                    val secretKey = SecretKey.fromByteArray(privateKeyBytes)
                    val publicKey = PublicKey.fromByteArray(publicKeyBytes)
                    keyPair = KeyPair(secretKey, publicKey)
                    Log.i(TAG, "sr25519 Identity keys loaded successfully. DID: ${getDID()}")
                } catch (e: Exception) {
                    Log.e(TAG, "Invalid key encoding in storage: ${e.message}. Re-generating...")
                    generateKeys()
                }
            }
        } catch (e: Exception) {
            Log.e(TAG, "Failed to initialize secure storage: ${e.message}")
            generateKeys()
        }
    }

    private fun generateKeys() {
        Log.i(TAG, "Generating new production sr25519 identity...")
        try {
            // In sr25519-kotlin, SecretKey.generate() handles secure entropy
            val secretKey = SecretKey.generate()
            val newKeyPair = secretKey.toKeyPair()
            keyPair = newKeyPair

            val privBytes = secretKey.toByteArray()
            val pubBytes = newKeyPair.publicKey.toByteArray()

            getEncryptedPrefs().edit()
                .putString(KEY_PRIV, Hex.toHexString(privBytes))
                .putString(KEY_PUB, Hex.toHexString(pubBytes))
                .apply()
            
            Log.i(TAG, "New identity generated. PubKey: ${Hex.toHexString(pubBytes).take(16)}...")
        } catch (e: Exception) {
            Log.e(TAG, "Key generation failed: ${e.message}", e)
        }
    }

    fun getPublicKeyHex(): String {
        return keyPair?.publicKey?.let { Hex.toHexString(it.toByteArray()) } ?: ""
    }

    fun getDID(): String {
        val address = getPublicKeyHex()
        if (address.isEmpty()) return ""
        return "did:peaq:0x$address"
    }

    fun sign(message: String): String {
        return signData(message.toByteArray())
    }

    fun signData(data: ByteArray): String {
        return try {
            val kp = keyPair ?: return ""
            // "substrate" matches the signing context used by Go schnorrkel (NewSigningContext("substrate", msg))
            val signature = kp.signSimple("substrate".toByteArray(), data)
            Hex.toHexString(signature.toByteArray())
        } catch (e: Exception) {
            Log.e(TAG, "Production signing failed: ${e.message}")
            ""
        }
    }
}
