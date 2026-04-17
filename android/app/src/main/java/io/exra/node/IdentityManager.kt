package io.exra.node

import android.content.Context
import android.util.Log
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKeys
import io.novasamatech.substrate_sdk_android.encrypt.keypair.Keypair
import io.novasamatech.substrate_sdk_android.encrypt.keypair.substrate.Sr25519KeypairFactory
import io.novasamatech.substrate_sdk_android.encrypt.SignatureWrapper
import io.novasamatech.substrate_sdk_android.encrypt.junction.Sr25519
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
            "/proc/tty/drivers" // checking for goldfish
        )
        for (p in probes) {
            if (File(p).exists()) {
                if (p == "/proc/tty/drivers") {
                    // Specific check inside goldfish driver info
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

    private var keypair: Keypair? = null
    private val keypairFactory = Sr25519KeypairFactory()

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
                generateKeys()
            } else {
                val privateKey = Hex.decode(privHex)
                val publicKey = Hex.decode(pubHex)
                keypair = Keypair(publicKey, privateKey)
                Log.i(TAG, "sr25519 Identity keys loaded from secure storage")
            }
        } catch (e: Exception) {
            Log.e(TAG, "Failed to initialize secure storage: ${e.message}")
            generateKeys()
        }
    }

    private fun clearLegacyStorage() {
        try {
            val oldPrefs = context.getSharedPreferences("ExraIdentity", Context.MODE_PRIVATE)
            if (oldPrefs.all.isNotEmpty()) {
                Log.i(TAG, "Clearing legacy insecure storage...")
                oldPrefs.edit().clear().apply()
            }
        } catch (e: Exception) {
            Log.w(TAG, "Failed to clear legacy storage: ${e.message}")
        }
    }

    private fun generateKeys() {
        Log.i(TAG, "Generating new sr25519 identity...")
        try {
            val seed = ByteArray(32)
            SecureRandom().nextBytes(seed)
            
            val newKeypair = keypairFactory.generate(seed, emptyList())
            keypair = newKeypair

            getEncryptedPrefs().edit()
                .putString(KEY_PRIV, Hex.toHexString(newKeypair.privateKey))
                .putString(KEY_PUB, Hex.toHexString(newKeypair.publicKey))
                .apply()
            
            Log.i(TAG, "New sr25519 identity generated: ${getPublicKeyHex().take(16)}...")
        } catch (e: Exception) {
            Log.e(TAG, "Key generation failed: ${e.message}")
        }
    }

    fun getPublicKeyHex(): String {
        return Hex.toHexString(keypair?.publicKey ?: byteArrayOf())
    }

    /**
     * Generates a peaq-compatible DID string from the public key.
     * Pattern: did:peaq:0x{hex_address}
     */
    fun getDID(): String {
        val pubBytes = keypair?.publicKey ?: return ""
        val address = Hex.toHexString(pubBytes)
        return "did:peaq:0x$address"
    }

    fun sign(message: String): String {
        return signData(message.toByteArray())
    }

    fun signData(data: ByteArray): String {
        return try {
            val kp = keypair ?: return ""
            val sig = Sr25519.sign(kp, data)
            Hex.toHexString(sig)
        } catch (e: Exception) {
            Log.e(TAG, "Signing failed: ${e.message}")
            ""
        }
    }
}
