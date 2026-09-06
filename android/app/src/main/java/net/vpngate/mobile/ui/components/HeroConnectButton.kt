package net.vpngate.mobile.ui.components

import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PowerSettingsNew
import androidx.compose.material3.Icon
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.unit.dp
import net.vpngate.mobile.data.model.ConnectionStatus
import net.vpngate.mobile.ui.theme.AppTheme
import net.vpngate.mobile.ui.theme.Cyan400
import net.vpngate.mobile.ui.theme.Cyan500
import net.vpngate.mobile.ui.theme.Emerald400
import net.vpngate.mobile.ui.theme.Emerald500

@Composable
fun HeroConnectButton(
    status: ConnectionStatus,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val colors = AppTheme.colors
    val isConnected = status == ConnectionStatus.CONNECTED
    val isConnecting = status == ConnectionStatus.CONNECTING

    val infiniteTransition = rememberInfiniteTransition(label = "pulse")

    val pulseScale by infiniteTransition.animateFloat(
        initialValue = 1f,
        targetValue = if (isConnected || isConnecting) 1.25f else 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 1800, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "pulseScale"
    )

    val pulseAlpha by infiniteTransition.animateFloat(
        initialValue = if (isConnected || isConnecting) 0.40f else 0f,
        targetValue = if (isConnected || isConnecting) 0.04f else 0f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 1800, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "pulseAlpha"
    )

    val activeGlowColor = when {
        isConnected -> if (colors.isDark) Emerald500 else Color(0xFF10B981)
        isConnecting -> if (colors.isDark) Cyan500 else Color(0xFF0284C7)
        else -> Color.Transparent
    }

    Box(
        contentAlignment = Alignment.Center,
        modifier = modifier.size(210.dp)
    ) {
        // Outer pulsing ring — uses graphicsLayer so recomposition doesn't run during animation
        if (isConnected || isConnecting) {
            Box(
                modifier = Modifier
                    .size(156.dp)
                    .graphicsLayer {
                        scaleX = pulseScale
                        scaleY = pulseScale
                        alpha = pulseAlpha
                    }
                    .background(activeGlowColor, CircleShape)
            )
        }

        // Outer Border Ring
        Canvas(modifier = Modifier.size(160.dp)) {
            val strokeColor = when {
                isConnected -> if (colors.isDark) Emerald400 else Color(0xFF059669)
                isConnecting -> if (colors.isDark) Cyan400 else Color(0xFF0284C7)
                else -> if (colors.isDark) Color(0xFF27272A) else Color(0xFFCBD5E1)
            }
            drawCircle(
                color = strokeColor,
                style = Stroke(width = 3.5.dp.toPx(), cap = StrokeCap.Round)
            )
        }

        // Inner Core Button
        val buttonGradient = when {
            isConnected -> if (colors.isDark) {
                Brush.verticalGradient(listOf(Color(0xFF064E3B), Color(0xFF18181B)))
            } else {
                Brush.verticalGradient(listOf(Color(0xFF10B981), Color(0xFF047857)))
            }
            isConnecting -> if (colors.isDark) {
                Brush.verticalGradient(listOf(Color(0xFF164E63), Color(0xFF18181B)))
            } else {
                Brush.verticalGradient(listOf(Color(0xFF0EA5E9), Color(0xFF0284C7)))
            }
            else -> if (colors.isDark) {
                Brush.verticalGradient(listOf(Color(0xFF27272A), Color(0xFF18181B)))
            } else {
                Brush.verticalGradient(listOf(Color(0xFFFFFFFF), Color(0xFFE2E8F0)))
            }
        }

        val buttonBorder = when {
            isConnected -> if (colors.isDark) Color(0xFF059669) else Color(0xFF047857)
            isConnecting -> if (colors.isDark) Color(0xFF0284C7) else Color(0xFF0369A1)
            else -> if (colors.isDark) Color(0xFF3F3F46) else Color(0xFFCBD5E1)
        }

        val iconTint = when {
            isConnected -> if (colors.isDark) Emerald400 else Color.White
            isConnecting -> if (colors.isDark) Cyan400 else Color.White
            else -> if (colors.isDark) Color(0xFF71717A) else Color(0xFF334155)
        }

        val shadowElevation = when {
            isConnected -> if (colors.isDark) 16.dp else 12.dp
            isConnecting -> if (colors.isDark) 14.dp else 10.dp
            else -> if (colors.isDark) 4.dp else 8.dp
        }

        val shadowColor = when {
            isConnected -> activeGlowColor
            isConnecting -> activeGlowColor
            else -> if (colors.isDark) Color.Black.copy(alpha = 0.3f) else Color(0x300F172A)
        }

        Box(
            contentAlignment = Alignment.Center,
            modifier = Modifier
                .size(136.dp)
                .shadow(
                    elevation = shadowElevation,
                    shape = CircleShape,
                    ambientColor = shadowColor,
                    spotColor = shadowColor
                )
                .clip(CircleShape)
                .background(buttonGradient)
                .border(1.5.dp, buttonBorder, CircleShape)
                .clickable(
                    interactionSource = remember { MutableInteractionSource() },
                    indication = null,
                    onClick = onClick
                )
                .padding(22.dp)
        ) {
            Icon(
                imageVector = Icons.Default.PowerSettingsNew,
                contentDescription = "Connect or Disconnect",
                tint = iconTint,
                modifier = Modifier.fillMaxSize()
            )
        }
    }
}
