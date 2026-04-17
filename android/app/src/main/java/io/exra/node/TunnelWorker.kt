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
    private val identityManager: PeaqIdentityWrapper
) {
    private val TAG = "TunnelWorker"

    suspend fun run() = withContext(Dispatchers.IO) {
        var serverSocket: Socket? = null
        var targetSocket: Socket? = null

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

            val timestamp = System.currentTimeMillis() / 1000
            val did = identityManager.getDid()
            val signature = identityManager.sign("$sessionId:$timestamp")

            val out = serverSocket.getOutputStream()
            // Manual HTTP Upgrade request with DePIN signatures
            val request = "GET /api/node/tunnel?session_id=$sessionId HTTP/1.1\r\n" +
                    "Host: $serverHost\r\n" +
                    "Upgrade: tcp-tunnel\r\n" +
                    "Connection: Upgrade\r\n" +
                    "X-DID: $did\r\n" +
                    "X-Timestamp: $timestamp\r\n" +
                    "X-Signature: $signature\r\n\r\n"
            out.write(request.toByteArray())
            out.flush()

            // 3. Bidirectional pipe
            val done = mutableListOf<Boolean>()
            
            val t1 = Thread {
                try {
                    pipe(serverSocket.getInputStream(), targetSocket.getOutputStream())
                } catch (e: Exception) {
                    Log.e(TAG, "Pipe server->target failed: ${e.message}")
                } finally {
                    synchronized(done) { done.add(true) }
                }
            }
            
            val t2 = Thread {
                try {
                    pipe(targetSocket.getInputStream(), serverSocket.getOutputStream())
                } catch (e: Exception) {
                    Log.e(TAG, "Pipe target->server failed: ${e.message}")
                } finally {
                    synchronized(done) { done.add(true) }
                }
            }

            t1.start()
            t2.start()

            // Wait for threads to finish or until one side closes or coroutine is cancelled
            while (isActive) {
                val isDone = synchronized(done) { done.size >= 1 }
                if (isDone) break
                Thread.sleep(200)
            }

        } catch (e: Exception) {
            Log.e(TAG, "Tunnel execution failed: ${e.message}", e)
        } finally {
            try { serverSocket?.close() } catch (e: Exception) {}
            try { targetSocket?.close() } catch (e: Exception) {}
            Log.d(TAG, "Tunnel closed for session $sessionId")
        }
    }

    private fun pipe(inputStream: InputStream, outputStream: OutputStream) {
        val buffer = ByteArray(32768)
        var bytesRead: Int
        while (true) {
            try {
                bytesRead = inputStream.read(buffer)
                if (bytesRead == -1) break
                outputStream.write(buffer, 0, bytesRead)
                outputStream.flush()
            } catch (e: Exception) {
                break
            }
        }
    }
}
