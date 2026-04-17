package io.exra.node

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log

class LinkRequestReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        val requestId = intent.getStringExtra("request_id") ?: return
        val approved = intent.action == "io.exra.node.ACTION_APPROVE"
        
        Log.d("LinkRequestReceiver", "Link request $requestId approved: $approved")
        
        // We need to tell the service to send the response
        val serviceIntent = Intent(context, NodeForegroundService::class.java).apply {
            action = "io.exra.node.ACTION_LINK_RESPONSE"
            putExtra("request_id", requestId)
            putExtra("approved", approved)
        }
        context.startService(serviceIntent)
    }
}
