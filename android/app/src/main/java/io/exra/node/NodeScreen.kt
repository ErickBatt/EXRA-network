package io.exra.node

import androidx.compose.foundation.layout.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

@Composable
fun NodeScreen(
    deviceId: String,
    did: String,
    isRunning: Boolean,
    lockStatus: String,
    timelockProgress: Int,
    onStart: () -> Unit,
    onStop: () -> Unit
) {
    Surface(
        modifier = Modifier.fillMaxSize(),
        color = DeepIndigo
    ) {
        Column(
            modifier = Modifier
                .padding(24.dp)
                .fillMaxSize(),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            // Header
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column {
                    Text(
                        text = "EXRA NODE",
                        color = Color.White,
                        fontSize = 20.sp,
                        fontWeight = FontWeight.Bold,
                        letterSpacing = 2.sp
                    )
                    Text(
                        text = "peaq DePIN Network",
                        color = ElectricGreen,
                        fontSize = 12.sp
                    )
                }
                StatusPulse(isRunning)
            }

            Spacer(Modifier.height(40.dp))

            // Stats Card
            GlassCard {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text("DEVICE ID", color = TextSecondary, fontSize = 10.sp)
                    Text(deviceId, color = Color.White, fontSize = 16.sp, fontWeight = FontWeight.Medium)
                }
            }

            Spacer(Modifier.height(24.dp))

            // Timelock Section
            Column(
                horizontalAlignment = Alignment.CenterHorizontally,
                modifier = Modifier.fillMaxWidth()
            ) {
                AnimatedTimelockCircle(
                    progress = timelockProgress,
                    title = "ANON TIMELOCK"
                )
                Spacer(Modifier.height(16.dp))
                Text(
                    text = lockStatus,
                    color = Color.White,
                    fontSize = 14.sp
                )
            }

            Spacer(Modifier.weight(1f))

            // Control Buttons
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(16.dp)
            ) {
                Button(
                    onClick = onStart,
                    enabled = !isRunning,
                    modifier = Modifier.weight(1f).height(56.dp),
                    colors = ButtonDefaults.buttonColors(containerColor = ElectricGreen),
                    shape = RoundedCornerShape(16.dp)
                ) {
                    Text("START", fontWeight = FontWeight.Bold)
                }
                
                OutlinedButton(
                    onClick = onStop,
                    enabled = isRunning,
                    modifier = Modifier.weight(1f).height(56.dp),
                    border = androidx.compose.foundation.BorderStroke(1.dp, Color.White.copy(alpha = 0.2f)),
                    shape = RoundedCornerShape(16.dp)
                ) {
                    Text("STOP", color = Color.White)
                }
            }

            Spacer(Modifier.height(24.dp))

            // Footer
            Text(
                text = "DID: $did",
                color = TextSecondary,
                fontSize = 10.sp,
                modifier = Modifier.padding(bottom = 8.dp)
            )
        }
    }
}
