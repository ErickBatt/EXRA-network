package io.exra.node

import android.Manifest
import android.content.Context
import android.content.DialogInterface
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.graphics.Color
import android.os.Build
import android.os.Bundle
import android.view.View
import android.widget.Toast
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.compose.runtime.*
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import android.os.PowerManager
import android.net.Uri
import android.provider.Settings
import org.json.JSONObject
import androidx.compose.runtime.mutableLongStateOf

class MainActivity : ComponentActivity() {
    private var isRunning by mutableStateOf(false)
    private var lockStatus by mutableStateOf("Identity: Anonymous")
    private var timelockProgress by mutableIntStateOf(0)
    private var activeTunnels by mutableIntStateOf(0)
    private var bytesProxied by mutableLongStateOf(0L)
    private var gearScore by mutableIntStateOf(0)
    private var pendingCredits by mutableStateOf("—")
    
    private fun checkEmulator(identityManager: IdentityManager) {
        if (identityManager.isEmulator()) {
            MaterialAlertDialogBuilder(this)
                .setTitle("Emulator Detected")
                .setMessage("EXRA Node can only run on physical devices for security and anti-fraud reasons.")
                .setCancelable(false)
                .setPositiveButton("Exit") { _: DialogInterface, _: Int -> finish() }
                .show()
        }
    }

    private fun requestBatteryOptimizationExemption() {
        val pm = getSystemService(POWER_SERVICE) as PowerManager
        if (!pm.isIgnoringBatteryOptimizations(packageName)) {
            MaterialAlertDialogBuilder(this)
                .setTitle("Background Activity")
                .setMessage("To ensure high GearScore and stable rewards, EXRA Node needs to run without battery optimizations. Please allow background activity in the next screen.")
                .setPositiveButton("Configure") { _: DialogInterface, _: Int ->
                    val intent = Intent(Settings.ACTION_REQUEST_IGNORE_BATTERY_OPTIMIZATIONS).apply {
                        data = Uri.parse("package:$packageName")
                    }
                    startActivity(intent)
                }
                .setNegativeButton("Later", null)
                .show()
        }
    }
    
    private val timelockReceiver = object : android.content.BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            val payloadStr = intent?.getStringExtra("payload") ?: return
            try {
                val payload = JSONObject(payloadStr)
                val unlockTs = payload.optLong("unlock_timestamp", 0)
                val currentTs = System.currentTimeMillis() / 1000
                
                if (unlockTs > currentTs) {
                    val remaining = unlockTs - currentTs
                    val hours = remaining / 3600
                    val minutes = (remaining % 3600) / 60
                    lockStatus = "Audit OK - Unlocking in ${hours}h ${minutes}m"
                    
                    val totalWindow = 86400L // 24h
                    val elapsed = totalWindow - remaining.coerceAtMost(totalWindow)
                    timelockProgress = ((elapsed.toDouble() / totalWindow) * 100).toInt()
                } else if (unlockTs > 0) {
                    lockStatus = "Status: Peak (Rewards Available)"
                    timelockProgress = 100
                }
            } catch (e: Exception) { /* ignore */ }
        }
    }

    private val linkRequestReceiver = object : android.content.BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            val user = intent?.getStringExtra("tg_user") ?: return
            val name = intent?.getStringExtra("tg_name") ?: return
            val reqId = intent?.getStringExtra("request_id") ?: return
            showLinkDialog(user, name, reqId)
        }
    }

    private val localStatsReceiver = object : android.content.BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            activeTunnels = intent?.getIntExtra("active_tunnels", 0) ?: 0
            bytesProxied = intent?.getLongExtra("bytes_proxied", 0L) ?: 0L
        }
    }

    private val nodeStatsReceiver = object : android.content.BroadcastReceiver() {
        override fun onReceive(context: Context?, intent: Intent?) {
            val payloadStr = intent?.getStringExtra("payload") ?: return
            try {
                val payload = JSONObject(payloadStr)
                if (payload.has("gear_score")) gearScore = payload.optInt("gear_score", 0)
                if (payload.has("pending_credits")) {
                    val credits = payload.optDouble("pending_credits", 0.0)
                    pendingCredits = "%.4f EXRA".format(credits)
                }
            } catch (e: Exception) { /* ignore */ }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        checkPermissions()

        val timelockFilter = IntentFilter("io.exra.node.TIMELOCK_UPDATE")
        val linkFilter = IntentFilter("io.exra.node.LINK_REQUEST")
        val localStatsFilter = IntentFilter("io.exra.node.LOCAL_STATS")
        val nodeStatsFilter = IntentFilter("io.exra.node.NODE_STATS")

        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            registerReceiver(timelockReceiver, timelockFilter, RECEIVER_NOT_EXPORTED)
            registerReceiver(linkRequestReceiver, linkFilter, RECEIVER_NOT_EXPORTED)
            registerReceiver(localStatsReceiver, localStatsFilter, RECEIVER_NOT_EXPORTED)
            registerReceiver(nodeStatsReceiver, nodeStatsFilter, RECEIVER_NOT_EXPORTED)
        } else {
            registerReceiver(timelockReceiver, timelockFilter)
            registerReceiver(linkRequestReceiver, linkFilter)
            registerReceiver(localStatsReceiver, localStatsFilter)
            registerReceiver(nodeStatsReceiver, nodeStatsFilter)
        }

        val prefs = getSharedPreferences("Exra", Context.MODE_PRIVATE)
        // Ensure device_id exists before the service starts so the UI shows it immediately.
        val deviceId = prefs.getString("device_id", null) ?: run {
            val generated = java.util.UUID.randomUUID().toString()
            prefs.edit().putString("device_id", generated).apply()
            generated
        }
        val identityManager = IdentityManager(this)

        // Security & Stability Checks
        checkEmulator(identityManager)
        requestBatteryOptimizationExemption()

        setContent {
            ExraTheme {
                NodeScreen(
                    deviceId = deviceId,
                    did = identityManager.getDID(),
                    isRunning = isRunning,
                    lockStatus = lockStatus,
                    timelockProgress = timelockProgress,
                    activeTunnels = activeTunnels,
                    bytesProxied = bytesProxied,
                    gearScore = gearScore,
                    pendingCredits = pendingCredits,
                    onStart = {
                        startNodeService()
                        isRunning = true
                    },
                    onStop = {
                        stopNodeService()
                        isRunning = false
                        timelockProgress = 0
                        lockStatus = "Identity: Anonymous"
                        activeTunnels = 0
                    }
                )
            }
        }
    }

    private fun startNodeService() {
        val intent = Intent(this, NodeForegroundService::class.java)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            startForegroundService(intent)
        } else {
            startService(intent)
        }
    }

    private fun stopNodeService() {
        stopService(Intent(this, NodeForegroundService::class.java))
    }

    private fun showLinkDialog(user: String, name: String, requestId: String) {
        MaterialAlertDialogBuilder(this)
            .setTitle("Device Link Request")
            .setMessage("Link this node to Telegram user $name (@$user)?")
            .setPositiveButton("Approve") { _: DialogInterface, _: Int ->
                val responseIntent = Intent(this, NodeForegroundService::class.java).apply {
                    action = "io.exra.node.ACTION_LINK_RESPONSE"
                    putExtra("request_id", requestId)
                    putExtra("approved", true)
                }
                startService(responseIntent)
                Toast.makeText(this, "Link approved", Toast.LENGTH_SHORT).show()
            }
            .setNegativeButton("Deny") { _: DialogInterface, _: Int ->
                val responseIntent = Intent(this, NodeForegroundService::class.java).apply {
                    action = "io.exra.node.ACTION_LINK_RESPONSE"
                    putExtra("request_id", requestId)
                    putExtra("approved", false)
                }
                startService(responseIntent)
            }
            .show()
    }

    override fun onDestroy() {
        super.onDestroy()
        unregisterReceiver(timelockReceiver)
        unregisterReceiver(linkRequestReceiver)
        unregisterReceiver(localStatsReceiver)
        unregisterReceiver(nodeStatsReceiver)
    }

    private fun checkPermissions() {
        val permissions = mutableListOf<String>()
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            permissions.add(Manifest.permission.POST_NOTIFICATIONS)
        }
        val toRequest = permissions.filter { 
            ContextCompat.checkSelfPermission(this, it) != PackageManager.PERMISSION_GRANTED 
        }
        if (toRequest.isNotEmpty()) {
            ActivityCompat.requestPermissions(this, toRequest.toTypedArray(), 101)
        }
    }
}
