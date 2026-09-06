package net.vpngate.mobile.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowDownward
import androidx.compose.material.icons.filled.ArrowUpward
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.LockOpen
import androidx.compose.material.icons.filled.Timer
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import net.vpngate.mobile.data.model.ConnectionStatus
import net.vpngate.mobile.ui.theme.Cyan400
import net.vpngate.mobile.ui.theme.Emerald400
import net.vpngate.mobile.ui.theme.Rose400
import net.vpngate.mobile.ui.theme.Zinc100
import net.vpngate.mobile.ui.theme.Zinc400
import net.vpngate.mobile.ui.theme.Zinc800
import net.vpngate.mobile.ui.theme.Zinc900
import java.util.Locale

@Composable
fun StatsBar(
    status: ConnectionStatus,
    durationSeconds: Long,
    bytesIn: Long,
    bytesOut: Long,
    modifier: Modifier = Modifier
) {
    val isConnected = status == ConnectionStatus.CONNECTED

    val minutes = durationSeconds / 60
    val seconds = durationSeconds % 60
    val durationText = if (isConnected) String.format("%02d:%02d", minutes, seconds) else "--:--"

    Box(
        modifier = modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(16.dp))
            .background(Zinc900)
            .border(1.dp, Zinc800, RoundedCornerShape(16.dp))
            .padding(14.dp)
    ) {
        Row(
            horizontalArrangement = Arrangement.SpaceAround,
            verticalAlignment = Alignment.CenterVertically,
            modifier = Modifier.fillMaxWidth()
        ) {
            StatItem(
                icon = Icons.Default.Timer,
                iconColor = if (isConnected) Emerald400 else Zinc400,
                label = "Duration",
                value = durationText
            )

            StatItem(
                icon = Icons.Default.ArrowDownward,
                iconColor = if (isConnected) Cyan400 else Zinc400,
                label = "Download",
                value = formatBytes(bytesIn)
            )

            StatItem(
                icon = Icons.Default.ArrowUpward,
                iconColor = if (isConnected) Cyan400 else Zinc400,
                label = "Upload",
                value = formatBytes(bytesOut)
            )

            StatItem(
                icon = if (isConnected) Icons.Default.Lock else Icons.Default.LockOpen,
                iconColor = if (isConnected) Emerald400 else Rose400,
                label = "Status",
                value = if (isConnected) "Protected" else "Exposed"
            )
        }
    }
}

@Composable
private fun StatItem(
    icon: ImageVector,
    iconColor: androidx.compose.ui.graphics.Color,
    label: String,
    value: String
) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        Icon(
            imageVector = icon,
            contentDescription = null,
            tint = iconColor,
            modifier = Modifier.size(16.dp)
        )
        Spacer(modifier = Modifier.height(3.dp))
        Text(
            text = value,
            color = Zinc100,
            fontSize = 12.sp,
            fontWeight = FontWeight.SemiBold
        )
        Text(
            text = label,
            color = Zinc400,
            fontSize = 10.sp
        )
    }
}

private fun formatBytes(bytes: Long): String {
    if (bytes <= 0) return "0 B"
    val kb = bytes / 1024.0
    val mb = kb / 1024.0
    return when {
        mb >= 1.0 -> String.format(Locale.US, "%.1f MB", mb)
        kb >= 1.0 -> String.format(Locale.US, "%.0f KB", kb)
        else -> "$bytes B"
    }
}
