package io.exra.node

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import androidx.core.app.NotificationCompat
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import okhttp3.OkHttpClient
import java.util.UUID
import java.util.Locale
import java.util.concurrent.TimeUnit
import android.telephony.TelephonyManager
import android.app.ActivityManager
import android.net.TrafficStats

class NodeForegroundService : Service() {
    private val scope = CoroutineScope(Dispatchers.IO + Job())
    private lateinit var wsClient: EXRAWsClient
    private lateinit var identityWrapper: PeaqIdentityWrapper

    override fun onCreate() {
        super.onCreate()
        val identityManager = IdentityManager(this)
        identityWrapper = PeaqIdentityWrapperImpl(identityManager)
        wsClient = EXRAWsClient(
            wsUrl = BuildConfig.EXRA_WS_URL,
            nodeSecret = BuildConfig.EXRA_NODE_SECRET,
            deviceIdProvider = { deviceId() },
            countryProvider = { getCountry() },
            deviceTypeProvider = { getDeviceType() },
            archProvider = { System.getProperty("os.arch") ?: "unknown" },
            ramProvider = { getRamMb() },
            trafficProvider = { getTrafficBytes() },
            pricePerGBProvider = { getUserPricePerGB() },
            okHttpClient = OkHttpClient.Builder()
                .pingInterval(30, TimeUnit.SECONDS)
                .build(),
            apiUrl = BuildConfig.EXRA_API_URL,
            identityManager = identityWrapper,
            onLinkRequest = { user, name, reqId -> 
                showLinkNotification(user, name, reqId)
                val intent = Intent("io.exra.node.LINK_REQUEST")
                intent.putExtra("tg_user", user)
                intent.putExtra("tg_name", name)
                intent.putExtra("request_id", reqId)
                sendBroadcast(intent)
            },
            onStatusUpdate = { payload ->
                val intent = Intent("io.exra.node.TIMELOCK_UPDATE")
                intent.putExtra("payload", payload.toString())
                sendBroadcast(intent)
            },
            onNodeStats = { payload ->
                val intent = Intent("io.exra.node.NODE_STATS")
                intent.putExtra("payload", payload.toString())
                sendBroadcast(intent)
            }
        )
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        if (intent?.action == "io.exra.node.ACTION_LINK_RESPONSE") {
            val reqId = intent.getStringExtra("request_id") ?: ""
            val approved = intent.getBooleanExtra("approved", false)
            wsClient.sendLinkResponse(reqId, approved)
            return START_STICKY
        }

        val notification = notification("Node service starting...")
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(1001, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
        } else {
            startForeground(1001, notification)
        }
        
        scope.launch {
            var backoffMs = 1000L
            while (isActive) {
                updateNotification("Node connected [Active]")

                // PoP Heartbeat loop (every 5 mins / 300s)
                val heartbeatJob = launch {
                    while (isActive) {
                        try {
                            wsClient.sendPopHeartbeat()
                        } catch (e: Exception) { /* ignore */ }
                        delay(300_000)
                    }
                }

                // Local stats broadcast (every 30s)
                val statsJob = launch {
                    while (isActive) {
                        delay(30_000)
                        broadcastLocalStats()
                    }
                }

                wsClient.connectAndRun()

                heartbeatJob.cancel()
                statsJob.cancel()
                updateNotification("Node disconnected. Retrying...")
                delay(backoffMs)
                backoffMs = (backoffMs * 2).coerceAtMost(30000L)
            }
        }
        return START_STICKY
    }

    private fun showLinkNotification(user: String, name: String, requestId: String) {
        val channelId = "Exra-link"
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(channelId, "Link Requests", NotificationManager.IMPORTANCE_HIGH)
            manager.createNotificationChannel(channel)
        }

        val approveIntent = Intent(this, LinkRequestReceiver::class.java).apply {
            action = "io.exra.node.ACTION_APPROVE"
            putExtra("request_id", requestId)
        }
        val approvePending = PendingIntent.getBroadcast(this, 1, approveIntent, PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)

        val denyIntent = Intent(this, LinkRequestReceiver::class.java).apply {
            action = "io.exra.node.ACTION_DENY"
            putExtra("request_id", requestId)
        }
        val denyPending = PendingIntent.getBroadcast(this, 2, denyIntent, PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_UPDATE_CURRENT)

        val notification = NotificationCompat.Builder(this, channelId)
            .setContentTitle("Link Request")
            .setContentText("Link device to Telegram user $name (@$user)?")
            .setSmallIcon(android.R.drawable.ic_dialog_info)
            .setPriority(NotificationCompat.PRIORITY_HIGH)
            .addAction(android.R.drawable.checkbox_on_background, "Approve", approvePending)
            .addAction(android.R.drawable.ic_menu_close_clear_cancel, "Deny", denyPending)
            .setAutoCancel(true)
            .build()

        manager.notify(2002, notification)
    }

    private fun updateNotification(text: String) {
        val manager = getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
        manager.notify(1001, notification(text))
    }

    override fun onDestroy() {
        wsClient.close()
        super.onDestroy()
    }

    override fun onBind(intent: Intent?): IBinder? = null

    private fun notification(text: String): Notification {
        val channelId = "Exra-node"
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(channelId, "Exra Node", NotificationManager.IMPORTANCE_LOW)
            getSystemService(NotificationManager::class.java).createNotificationChannel(channel)
        }
        return NotificationCompat.Builder(this, channelId)
            .setContentTitle("Exra Node")
            .setContentText(text)
            .setSmallIcon(android.R.drawable.stat_notify_sync)
            .build()
    }

    private fun deviceId(): String {
        val prefs = getSharedPreferences("Exra", Context.MODE_PRIVATE)
        val existing = prefs.getString("device_id", null)
        if (existing != null) return existing
        val generated = UUID.randomUUID().toString()
        prefs.edit().putString("device_id", generated).apply()
        return generated
    }

    private fun getCountry(): String {
        val tm = getSystemService(Context.TELEPHONY_SERVICE) as? TelephonyManager
        val country = tm?.networkCountryIso
        if (!country.isNullOrEmpty()) {
            return country.uppercase(Locale.getDefault())
        }
        return Locale.getDefault().country.uppercase(Locale.getDefault())
    }

    private fun getDeviceType(): String {
        return "${Build.MANUFACTURER} ${Build.MODEL}"
    }

    private fun getRamMb(): Int {
        val actManager = getSystemService(Context.ACTIVITY_SERVICE) as ActivityManager
        val memInfo = ActivityManager.MemoryInfo()
        actManager.getMemoryInfo(memInfo)
        return (memInfo.totalMem / (1024 * 1024)).toInt()
    }

    private fun getTrafficBytes(): Long {
        val uid = android.os.Process.myUid()
        val rx = TrafficStats.getUidRxBytes(uid)
        val tx = TrafficStats.getUidTxBytes(uid)
        return (if (rx == TrafficStats.UNSUPPORTED.toLong()) 0 else rx) + 
               (if (tx == TrafficStats.UNSUPPORTED.toLong()) 0 else tx)
    }

    private fun getUserPricePerGB(): Double {
        val prefs = getSharedPreferences("Exra", Context.MODE_PRIVATE)
        return prefs.getFloat("user_price_per_gb", 0f).toDouble()
    }

    private fun broadcastLocalStats() {
        val intent = Intent("io.exra.node.LOCAL_STATS")
        intent.putExtra("active_tunnels", wsClient.getActiveTunnels())
        intent.putExtra("bytes_proxied", wsClient.getTotalBytesProxied())
        sendBroadcast(intent)
    }
}
