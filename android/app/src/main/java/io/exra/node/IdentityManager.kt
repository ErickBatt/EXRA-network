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
import java.security.SecureRandom

class IdentityManager(private val context: Context) {
    private val TAG = "IdentityManager"
    private val PREFS_NAME = "ExraIdentitySecure"
    private val KEY_PRIV = "priv_key"
    private val KEY_PUB = "pub_key"

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
        // Use SS58-like or simple hex for peaq DID address part
        val address = Hex.toHexString(pubBytes).take(40) // Take 20 bytes equivalent
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
