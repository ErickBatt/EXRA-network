package io.exra.node

import android.util.Log
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import kotlinx.coroutines.isActive
import java.io.InputStream
import java.io.OutputStream
import java.net.InetSocketAddress
import java.net.Socket
import java.net.URL
import javax.net.SocketFactory
import javax.net.ssl.SSLSocketFactory

class TunnelWorker(
    private val apiUrl: String,
    private val sessionId: String,
    private val targetHost: String,
    private val targetPort: Int,
    private val identityManager: PeaqIdentityWrapper,
    private val deviceId: String,
    private val onComplete: (bytesProxied: Long) -> Unit = {}
) {
    private val TAG = "TunnelWorker"

    suspend fun run() = withContext(Dispatchers.IO) {
        var serverSocket: Socket? = null
        var targetSocket: Socket? = null
        var totalBytes = 0L

        try {
            Log.d(TAG, "Opening tunnel for session $sessionId to $targetHost:$targetPort")

            // 1. Connect to the target (destination the buyer wants to reach)
            targetSocket = SocketFactory.getDefault().createSocket()
            targetSocket.connect(InetSocketAddress(targetHost, targetPort), 10000)

            // 2. Connect to the server's tunnel endpoint
            val serverUri = URL(apiUrl)
            val serverHost = serverUri.host
            val isHttps = serverUri.protocol.equals("https", ignoreCase = true)
            val serverPort = if (serverUri.port != -1) serverUri.port else serverUri.defaultPort

            serverSocket = if (isHttps) {
                SSLSocketFactory.getDefault().createSocket(serverHost, serverPort)
            } else {
                val s = SocketFactory.getDefault().createSocket()
                s.connect(InetSocketAddress(serverHost, serverPort), 10000)
                s
            }

            // TunnelHandler requires X-Device-ID + X-Device-Sig (sr25519 sig over raw sessionId only).
            // No timestamp in the signed payload — server checks DID ownership, not replay window here.
            val signature = identityManager.sign(sessionId)

            val out = serverSocket.getOutputStream()
            val request = "GET /api/node/tunnel?session_id=$sessionId HTTP/1.1\r\n" +
                    "Host: $serverHost\r\n" +
                    "Connection: close\r\n" +
                    "X-Device-ID: $deviceId\r\n" +
                    "X-Device-Sig: $signature\r\n\r\n"
            out.write(request.toByteArray())
            out.flush()

            // 3. Bidirectional pipe — both threads count bytes independently
            val done = mutableListOf<Boolean>()
            var bytesS2T = 0L
            var bytesT2S = 0L

            val t1 = Thread {
                try {
                    bytesS2T = pipe(serverSocket.getInputStream(), targetSocket.getOutputStream())
                } catch (e: Exception) {
                    Log.e(TAG, "Pipe server->target failed: ${e.message}")
                } finally {
                    synchronized(done) { done.add(true) }
                }
            }

            val t2 = Thread {
                try {
                    bytesT2S = pipe(targetSocket.getInputStream(), serverSocket.getOutputStream())
                } catch (e: Exception) {
                    Log.e(TAG, "Pipe target->server failed: ${e.message}")
                } finally {
                    synchronized(done) { done.add(true) }
                }
            }

            t1.start()
            t2.start()

            // Wait until BOTH pipe threads finish (or coroutine is cancelled).
            // Exiting on the first completion would close both sockets while the
            // second thread still has bytes in flight, causing data loss.
            while (isActive) {
                val isDone = synchronized(done) { done.size >= 2 }
                if (isDone) break
                Thread.sleep(200)
            }

            totalBytes = bytesS2T + bytesT2S

        } catch (e: Exception) {
            Log.e(TAG, "Tunnel execution failed: ${e.message}", e)
        } finally {
            try { serverSocket?.close() } catch (e: Exception) {}
            try { targetSocket?.close() } catch (e: Exception) {}
            Log.d(TAG, "Tunnel closed for session $sessionId bytes=$totalBytes")
            onComplete(totalBytes)
        }
    }

    private fun pipe(inputStream: InputStream, outputStream: OutputStream): Long {
        val buffer = ByteArray(32768)
        var total = 0L
        while (true) {
            try {
                val n = inputStream.read(buffer)
                if (n == -1) break
                outputStream.write(buffer, 0, n)
                outputStream.flush()
                total += n
            } catch (e: Exception) {
                break
            }
        }
        return total
    }
}
