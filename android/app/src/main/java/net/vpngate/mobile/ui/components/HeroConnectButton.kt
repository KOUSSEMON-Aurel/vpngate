package net.vpngate.mobile.ui.components

import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
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
import androidx.compose.ui.unit.dp
import net.vpngate.mobile.data.model.ConnectionStatus
import net.vpngate.mobile.ui.theme.Cyan500
import net.vpngate.mobile.ui.theme.Emerald400
import net.vpngate.mobile.ui.theme.Emerald500
import net.vpngate.mobile.ui.theme.Rose500
import net.vpngate.mobile.ui.theme.Zinc800
import net.vpngate.mobile.ui.theme.Zinc900

@Composable
fun HeroConnectButton(
    status: ConnectionStatus,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val isConnected = status == ConnectionStatus.CONNECTED
    val isConnecting = status == ConnectionStatus.CONNECTING

    val infiniteTransition = rememberInfiniteTransition(label = "pulse")

    val pulseScale by infiniteTransition.animateFloat(
        initialValue = 1f,
        targetValue = if (isConnected || isConnecting) 1.22f else 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 1800, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "pulseScale"
    )

    val pulseAlpha by infiniteTransition.animateFloat(
        initialValue = if (isConnected || isConnecting) 0.45f else 0f,
        targetValue = if (isConnected || isConnecting) 0.05f else 0f,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMillis = 1800, easing = FastOutSlowInEasing),
            repeatMode = RepeatMode.Reverse
        ),
        label = "pulseAlpha"
    )

    val activeGlowColor = when {
        isConnected -> Emerald500
        isConnecting -> Cyan500
        else -> Color.Transparent
    }

    Box(
        contentAlignment = Alignment.Center,
        modifier = modifier.size(210.dp)
    ) {
        // Outer pulsing ring
        if (isConnected || isConnecting) {
            Canvas(
                modifier = Modifier
                    .size(175.dp * pulseScale)
            ) {
                drawCircle(
                    color = activeGlowColor.copy(alpha = pulseAlpha)
                )
            }
        }

        // Outer Border Ring
        Canvas(modifier = Modifier.size(160.dp)) {
            val strokeColor = when {
                isConnected -> Emerald400
                isConnecting -> Cyan500
                else -> Zinc800
            }
            drawCircle(
                color = strokeColor,
                style = Stroke(width = 3.dp.toPx(), cap = StrokeCap.Round)
            )
        }

        // Inner Core Button
        val buttonGradient = when {
            isConnected -> Brush.verticalGradient(listOf(Color(0xFF064E3B), Zinc900))
            isConnecting -> Brush.verticalGradient(listOf(Color(0xFF164E63), Zinc900))
            else -> Brush.verticalGradient(listOf(Zinc800, Zinc900))
        }

        val iconTint = when {
            isConnected -> Emerald400
            isConnecting -> Cyan500
            else -> Color(0xFF71717A)
        }

        Box(
            contentAlignment = Alignment.Center,
            modifier = Modifier
                .size(136.dp)
                .shadow(
                    elevation = if (isConnected) 16.dp else 4.dp,
                    shape = CircleShape,
                    ambientColor = activeGlowColor,
                    spotColor = activeGlowColor
                )
                .clip(CircleShape)
                .background(buttonGradient)
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
