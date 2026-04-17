package io.exra.node

interface PeaqIdentityWrapper {
    fun getDid(): String
    fun getPublicKey(): String
    fun sign(data: ByteArray): String
    fun sign(message: String): String
}

class PeaqIdentityWrapperImpl(private val manager: IdentityManager) : PeaqIdentityWrapper {
    override fun getDid(): String = manager.getDID()
    
    override fun getPublicKey(): String = manager.getPublicKeyHex()
    
    override fun sign(data: ByteArray): String {
        val signature = manager.signData(data)
        return if (signature.startsWith("0x")) signature else "0x$signature"
    }
    
    override fun sign(message: String): String = sign(message.toByteArray())
}
