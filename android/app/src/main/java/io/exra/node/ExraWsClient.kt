package io.exra.node

import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.Response
import okhttp3.WebSocket
import okhttp3.WebSocketListener
import org.json.JSONObject
import java.util.UUID
import java.time.Instant
import java.time.format.DateTimeFormatter
import java.time.ZoneOffset
import android.util.Log
import kotlin.random.Random

class EXRAWsClient(
    private val wsUrl: String,
    private val nodeSecret: String,
    private val deviceIdProvider: () -> String,
    private val countryProvider: () -> String,
    private val deviceTypeProvider: () -> String,
    private val archProvider: () -> String,
    private val ramProvider: () -> Int,
    private val trafficProvider: () -> Long,
    private val pricePerGBProvider: () -> Double,
    private val okHttpClient: OkHttpClient,
    private val apiUrl: String,
    private val identityManager: PeaqIdentityWrapper,
    private val onLinkRequest: (String, String, String) -> Unit = { _, _, _ -> },
    private val onStatusUpdate: (JSONObject) -> Unit = {}
) {
    private var ws: WebSocket? = null

    suspend fun connectAndRun() {
        val deviceId = deviceIdProvider()
        val timestamp = System.currentTimeMillis() / 1000
        
        val did = identityManager.getDid()
        val signature = identityManager.sign("$deviceId:$did:$timestamp")

        val done = CompletableDeferred<Unit>()
        val request = Request.Builder()
            .url(wsUrl)
            .header("X-Device-ID", deviceId)
            .header("X-DID", did)
            .header("X-Timestamp", timestamp.toString())
            .header("X-Signature", signature)
            .build()
        ws = okHttpClient.newWebSocket(request, object : WebSocketListener() {
            override fun onOpen(webSocket: WebSocket, response: Response) {
                val register = JSONObject()
                    .put("type", "register")
                    .put("device_id", deviceIdProvider())
                    .put("country", countryProvider())
                    .put("device_type", deviceTypeProvider())
                    .put("arch", archProvider())
                    .put("ram_mb", ramProvider())
                    .put("did", identityManager.getDid())
                    .put("public_key", identityManager.getPublicKey())
                    .put("price_per_gb", pricePerGBProvider())
                    .put("auto_price", pricePerGBProvider() <= 0.0)
                    .put("signature", signature)
                    .put("timestamp", timestamp.toString())
                webSocket.send(register.toString())
                
                // Traffic reporting REMOVED as per v2.1 (PoP Heartbeats used instead)
            }

            override fun onMessage(webSocket: WebSocket, text: String) {
                try {
                    val payload = JSONObject(text)
                    val type = payload.optString("type")
                    
                    when (type) {
                        "ping" -> {
                            webSocket.send(JSONObject().put("type", "pong").toString())
                        }
                        "proxy_open" -> {
                            val sessionId = payload.optString("session_id")
                            val targetHost = payload.optString("target_host")
                            val targetPort = payload.optInt("target_port")
                            
                            if (sessionId.isNotEmpty() && targetHost.isNotEmpty()) {
                                CoroutineScope(Dispatchers.IO).launch {
                                    val worker = TunnelWorker(apiUrl, sessionId, targetHost, targetPort)
                                    worker.run()
                                }
                            }
                        }
                        "link_request" -> {
                            val tgUser = payload.optString("tg_user")
                            val tgName = payload.optString("tg_first_name")
                            val requestId = payload.optString("request_id")
                            onLinkRequest(tgUser, tgName, requestId)
                        }
                        "timelock_update" -> {
                            onStatusUpdate(payload)
                        }
                        "compute_task" -> {
                            val taskId = payload.optString("task_id")
                            CoroutineScope(Dispatchers.Default).launch {
                                // Simulate heavy work
                                delay(Random.nextLong(2000, 5000))
                                
                                val resultValue = JSONObject().put("computed_value", 42).put("status", "ok")
                                val resultString = resultValue.toString()
                                
                                // ZK-light Attestation
                                val md = java.security.MessageDigest.getInstance("SHA-256")
                                val resBytes = md.digest(resultString.toByteArray())
                                val resultHash = resBytes.joinToString("") { "%02x".format(it) }
                                
                                val ts = System.currentTimeMillis() / 1000
                                val payloadToSign = "$taskId:$resultHash:$ts"
                                val sig = identityManager.sign(payloadToSign)
                                
                                val resultMsg = JSONObject()
                                    .put("type", "compute_result")
                                    .put("data", JSONObject()
                                        .put("task_id", taskId)
                                        .put("result", resultValue)
                                        .put("attestation", JSONObject()
                                            .put("result_hash", resultHash)
                                            .put("timestamp", ts)
                                            .put("did", identityManager.getDid())
                                            .put("signature", sig)
                                        )
                                    )
                                webSocket.send(resultMsg.toString())
                                Log.i("EXRA", "[ZK-light] Sent signed result for task $taskId")
                            }
                        }
                    }
                } catch (e: Exception) {
                    Log.e("EXRA", "WS Message handling failed: ${e.message}")
                }
            }

            override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
                done.complete(Unit)
            }

            override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
                done.complete(Unit)
            }
        })
        done.await()
    }

    fun sendPopHeartbeat() {
        val ts = System.currentTimeMillis() / 1000
        val did = identityManager.getDid()
        val signature = identityManager.sign(ts.toString())
        
        val heartbeat = JSONObject()
            .put("type", "heartbeat")
            .put("data", JSONObject()
                .put("did", did)
                .put("timestamp", ts)
                .put("signature", signature)
            )
        ws?.send(heartbeat.toString())
    }

    fun sendLinkResponse(requestId: String, approved: Boolean) {
        val resp = JSONObject()
            .put("type", "link_response")
            .put("request_id", requestId)
            .put("approved", approved)
        ws?.send(resp.toString())
    }

    private suspend fun performFeederCheck(ip: String, port: Int): String = kotlinx.coroutines.withContext(Dispatchers.IO) {
        try {
            // Simple reachability check: can we connect to the proxy port?
            val client = OkHttpClient.Builder()
                .connectTimeout(5, java.util.concurrent.TimeUnit.SECONDS)
                .build()
            
            // In production, we check /health or similar. 
            // For MVP, just hitting the node is enough.
            val request = Request.Builder()
                .url("http://$ip:$port/health")
                .build()
                
            client.newCall(request).execute().use { response ->
                if (response.isSuccessful) "honest" else "fraud"
            }
        } catch (e: Exception) {
            Log.w("EXRA", "[Feeder] Audit failed for $ip: ${e.message}")
            "fraud" // If unreachable, mark as fraud
        }
    }

    fun close() {
        ws?.close(1000, "service stop")
    }
}
