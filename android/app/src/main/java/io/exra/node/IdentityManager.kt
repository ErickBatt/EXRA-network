package io.exra.node

import android.content.Context
import android.util.Log
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKeys
import io.novasama.substrate_sdk_android.encrypt.keypair.KeyPair
import io.novasama.substrate_sdk_android.extensions.toHexString
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
            val nonceHex = prefs.getString(KEY_NONCE, null)

            if (privHex == null || pubHex == null) {
                generateKeys()
            } else {
                val privateKey = Hex.decode(privHex)
                val publicKey = Hex.decode(pubHex)
                val nonce = nonceHex?.let { Hex.decode(it) }
                
                // KeyPair from bytes (Substrate SDK style)
                keyPair = KeyPair(publicKey, privateKey, nonce)
                Log.i(TAG, "sr25519 Identity keys loaded from secure storage (Production SDK)")
            }
        } catch (e: Exception) {
            Log.e(TAG, "Failed to initialize secure storage: ${e.message}")
            generateKeys()
        }
    }

    private fun generateKeys() {
        Log.i(TAG, "Generating new production sr25519 identity...")
        try {
            // Securely generate a 32-byte seed
            val seed = ByteArray(32)
            SecureRandom().nextBytes(seed)
            val seedHex = seed.toHexString(withPrefix = true)
            
            // Use the hardened Silencio/Nova Factory
            val newKeyPair = KeyPair.Factory.sr25519().generate(seedHex)
            keyPair = newKeyPair

            getEncryptedPrefs().edit()
                .putString(KEY_PRIV, newKeyPair.privateKey.toHexString())
                .putString(KEY_PUB, newKeyPair.publicKey.toHexString())
                .putString(KEY_NONCE, newKeyPair.nonce?.toHexString())
                .apply()
            
            Log.i(TAG, "New production identity generated: ${getPublicKeyHex().take(16)}...")
        } catch (e: Exception) {
            Log.e(TAG, "Production key generation failed: ${e.message}")
        }
    }

    fun getPublicKeyHex(): String {
        return keyPair?.publicKey?.toHexString() ?: ""
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
            // Use the production-grade sign method
            val signature = kp.sign(data)
            signature.toHexString(withPrefix = true)
        } catch (e: Exception) {
            Log.e(TAG, "Production signing failed: ${e.message}")
            ""
        }
    }
}
