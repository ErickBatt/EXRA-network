package io.exra.node

import android.Manifest
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.TextView
import android.widget.Toast
import android.content.IntentFilter
import android.graphics.Color
import android.view.View
import android.widget.*
import androidx.appcompat.app.AppCompatActivity
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import androidx.appcompat.app.AlertDialog
import com.google.android.material.dialog.MaterialAlertDialogBuilder
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.util.UUID

class MainActivity : AppCompatActivity() {
    private val http = OkHttpClient()
    private lateinit var statusText: TextView
    private lateinit var lockStatus: TextView
    private lateinit var timelockProgress: ProgressBar
    
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
                    lockStatus.text = "Audit OK - Unlocking in ${hours}h ${minutes}m"
                    lockStatus.setTextColor(Color.parseColor("#FFA500")) // Orange
                    
                    val totalWindow = 86400L // 24h
                    val elapsed = totalWindow - remaining.coerceAtMost(totalWindow)
                    val progress = ((elapsed.toDouble() / totalWindow) * 100).toInt()
                    timelockProgress.visibility = View.VISIBLE
                    timelockProgress.progress = progress
                } else if (unlockTs > 0) {
                    lockStatus.text = "Status: Peak (Rewards Available)"
                    lockStatus.setTextColor(Color.parseColor("#22BB33")) // Green
                    timelockProgress.progress = 100
                }
            } catch (e: Exception) {
                // parse error
            }
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

    private fun showLinkDialog(user: String, name: String, requestId: String) {
        MaterialAlertDialogBuilder(this)
            .setTitle("Device Link Request")
            .setMessage("Link this node to Telegram user $name (@$user)?")
            .setPositiveButton("Approve") { _, _ ->
                val responseIntent = Intent(this, NodeForegroundService::class.java).apply {
                    action = "io.exra.node.ACTION_LINK_RESPONSE"
                    putExtra("request_id", requestId)
                    putExtra("approved", true)
                }
                startService(responseIntent)
                Toast.makeText(this, "Link approved", Toast.LENGTH_SHORT).show()
            }
            .setNegativeButton("Deny") { _, _ ->
                val responseIntent = Intent(this, NodeForegroundService::class.java).apply {
                    action = "io.exra.node.ACTION_LINK_RESPONSE"
                    putExtra("request_id", requestId)
                    putExtra("approved", false)
                }
                startService(responseIntent)
            }
            .show()
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        checkPermissions()
        
        val timelockFilter = IntentFilter("io.exra.node.TIMELOCK_UPDATE")
        val linkFilter = IntentFilter("io.exra.node.LINK_REQUEST")
        
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            registerReceiver(timelockReceiver, timelockFilter, RECEIVER_EXPORTED)
            registerReceiver(linkRequestReceiver, linkFilter, RECEIVER_EXPORTED)
        } else {
            registerReceiver(timelockReceiver, timelockFilter)
            registerReceiver(linkRequestReceiver, linkFilter)
        }

        val prefs = getSharedPreferences("Exra", Context.MODE_PRIVATE)
        val savedId = prefs.getString("device_id", "Unknown") ?: "Unknown"
        val identityManager = IdentityManager(this)

        statusText = TextView(this).apply {
            text = "Exra Node (peaq DePIN)\nDevice ID: $savedId\nStatus: ready"
            textSize = 20f
            setTextColor(Color.parseColor("#1A237E")) 
            setPadding(60, 60, 60, 20)
        }
        
        lockStatus = TextView(this).apply {
            text = "Identity: Anonymous"
            textSize = 15f
            setPadding(60, 0, 60, 10)
        }
        
        timelockProgress = ProgressBar(this, null, android.R.attr.progressBarStyleHorizontal).apply {
            max = 100
            progress = 0
            visibility = View.GONE
            setPadding(60, 10, 60, 40)
        }

        val start = Button(this).apply {
            text = "Start Node"
            setTextColor(Color.WHITE)
            setBackgroundColor(Color.parseColor("#4CAF50"))
            setOnClickListener {
                val intent = Intent(this@MainActivity, NodeForegroundService::class.java)
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                    startForegroundService(intent)
                } else {
                    startService(intent)
                }
                statusText.text = "Exra Node (peaq DePIN)\nDevice ID: $savedId\nStatus: running"
            }
        }
        
        val stop = Button(this).apply {
            text = "Stop Node"
            setTextColor(Color.WHITE)
            setBackgroundColor(Color.parseColor("#F44336"))
            setOnClickListener {
                stopService(Intent(this@MainActivity, NodeForegroundService::class.java))
                statusText.text = "Exra Node (peaq DePIN)\nDevice ID: $savedId\nStatus: inactive"
                timelockProgress.visibility = View.GONE
                lockStatus.text = "Identity: Anonymous"
                lockStatus.setTextColor(Color.BLACK)
            }
        }

        val footer = TextView(this).apply {
            text = "DID: ${identityManager.getDID()}"
            textSize = 11f
            setPadding(60, 80, 60, 40)
        }

        val mainLayout = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setBackgroundColor(Color.parseColor("#FAFAFA"))
            addView(statusText)
            addView(lockStatus)
            addView(timelockProgress)
            
            val buttonRow = LinearLayout(this@MainActivity).apply {
                orientation = LinearLayout.HORIZONTAL
                setPadding(60, 40, 60, 20)
                addView(start, LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1.0f).apply { marginEnd = 10 })
                addView(stop, LinearLayout.LayoutParams(0, LinearLayout.LayoutParams.WRAP_CONTENT, 1.0f).apply { marginStart = 10 })
            }
            addView(buttonRow)
            addView(footer)
        }
        setContentView(mainLayout)
    }

    override fun onDestroy() {
        super.onDestroy()
        unregisterReceiver(timelockReceiver)
        unregisterReceiver(linkRequestReceiver)
    }

    private suspend fun performPrecheck(token: String, payload: JSONObject): String = withContext(Dispatchers.IO) {
        val req = Request.Builder()
            .url("${BuildConfig.EXRA_API_URL}/api/payout/precheck")
            .addHeader("X-Exra-Token", token)
            .post(payload.toString().toRequestBody("application/json".toMediaType()))
            .build()
        
        http.newCall(req).execute().use { resp ->
            val body = resp.body?.string().orEmpty()
            if (!resp.isSuccessful) {
                "Precheck failed (${resp.code}): $body"
            } else {
                val json = JSONObject(body)
                val net = json.optDouble("net_amount_usd")
                "Net amount: $net USD (peaq)"
            }
        }
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
