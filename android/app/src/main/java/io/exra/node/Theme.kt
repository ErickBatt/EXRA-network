package io.exra.node

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

val DeepIndigo = Color(0xFF0A0E21)
val ElectricGreen = Color(0xFF00FF94)
val NeonPurple = Color(0xFFBD00FF)
val GlassWhite = Color(0x1AFFFFFF)
val TextSecondary = Color(0xFF94A3B8)

private val DarkColorScheme = darkColorScheme(
    primary = ElectricGreen,
    secondary = NeonPurple,
    background = DeepIndigo,
    surface = Color(0xFF1E293B),
    onPrimary = Color.Black,
    onBackground = Color.White,
    onSurface = Color.White
)

@Composable
fun ExraTheme(
    content: @Composable () -> Unit
) {
    MaterialTheme(
        colorScheme = DarkColorScheme,
        content = content
    )
}
