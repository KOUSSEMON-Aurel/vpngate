package net.vpngate.mobile.ui.components

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import net.vpngate.mobile.data.model.VpnServer
import net.vpngate.mobile.ui.theme.Amber500
import net.vpngate.mobile.ui.theme.AppTheme
import net.vpngate.mobile.ui.theme.Rose500

@Composable
fun ServerCard(
    server: VpnServer,
    isSelected: Boolean,
    isCurrentConnected: Boolean,
    onSelect: () -> Unit,
    onConnect: () -> Unit,
    modifier: Modifier = Modifier
) {
    val colors = AppTheme.colors
    val strings = AppTheme.strings

    val borderColor = when {
        isCurrentConnected -> colors.statusConnected
        isSelected -> colors.accentSecondary
        else -> colors.border
    }

    val pingColor = when {
        server.ping > 8000 -> colors.textMuted
        server.ping <= 60 -> colors.statusConnected
        server.ping <= 140 -> Amber500
        else -> Rose500
    }

    Box(
        modifier = modifier
            .fillMaxWidth()
            .shadow(
                elevation = if (colors.isDark) 0.dp else 3.dp,
                shape = RoundedCornerShape(16.dp),
                ambientColor = colors.cardShadowColor,
                spotColor = colors.cardShadowColor
            )
            .clip(RoundedCornerShape(16.dp))
            .background(colors.surface)
            .border(1.dp, borderColor, RoundedCornerShape(16.dp))
            .clickable(onClick = onSelect)
            .padding(14.dp)
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween,
            modifier = Modifier.fillMaxWidth()
        ) {
            // Country flag + Server Info
            Row(
                verticalAlignment = Alignment.CenterVertically,
                modifier = Modifier.weight(1f)
            ) {
                Box(
                    modifier = Modifier
                        .size(38.dp)
                        .clip(RoundedCornerShape(8.dp))
                        .background(if (colors.isDark) colors.surfaceVariant else Color(0xFFE0F2FE))
                        .border(1.dp, if (colors.isDark) colors.border else Color(0xFFBAE6FD), RoundedCornerShape(8.dp)),
                    contentAlignment = Alignment.Center
                ) {
                    Text(
                        text = server.countryBadge,
                        color = colors.accentSecondary,
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Bold
                    )
                }
                Spacer(modifier = Modifier.width(12.dp))

                Column(modifier = Modifier.weight(1f)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Text(
                            text = server.countryLong,
                            color = colors.textPrimary,
                            fontWeight = FontWeight.SemiBold,
                            fontSize = 15.sp,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        if (isCurrentConnected) {
                            Spacer(modifier = Modifier.width(6.dp))
                            Icon(
                                imageVector = Icons.Default.CheckCircle,
                                contentDescription = "Active Server",
                                tint = colors.statusConnected,
                                modifier = Modifier.size(16.dp)
                            )
                        }
                    }

                    Spacer(modifier = Modifier.height(2.dp))

                    Text(
                        text = server.ip,
                        color = colors.textSecondary,
                        fontSize = 12.sp,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )

                    Spacer(modifier = Modifier.height(4.dp))

                    // Metrics badge row
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Box(
                            modifier = Modifier
                                .size(6.dp)
                                .clip(CircleShape)
                                .background(pingColor)
                        )
                        Spacer(modifier = Modifier.width(4.dp))
                        Text(
                            text = if (server.ping > 8000) "Offline" else "${server.ping} ms",
                            color = pingColor,
                            fontSize = 11.sp,
                            fontWeight = FontWeight.Medium
                        )

                        Spacer(modifier = Modifier.width(10.dp))

                        Box(
                            modifier = Modifier
                                .clip(RoundedCornerShape(4.dp))
                                .background(if (server.isWarp) colors.accentSecondary.copy(alpha = 0.15f) else colors.surfaceVariant)
                                .padding(horizontal = 6.dp, vertical = 2.dp)
                        ) {
                            Text(
                                text = if (server.isWarp) "WARP WireGuard" else "OpenVPN",
                                color = if (server.isWarp) colors.accentSecondary else colors.textSecondary,
                                fontSize = 10.sp,
                                fontWeight = FontWeight.SemiBold
                            )
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.width(8.dp))

            // Connect button
            val btnBg = when {
                isCurrentConnected -> if (colors.isDark) Color(0xFF064E3B) else Color(0xFFD1FAE5)
                else -> if (colors.isDark) colors.surfaceVariant else Color(0xFFF1F5F9)
            }
            val btnFg = when {
                isCurrentConnected -> if (colors.isDark) colors.statusConnected else Color(0xFF065F46)
                else -> colors.textPrimary
            }

            FilledTonalButton(
                onClick = onConnect,
                colors = ButtonDefaults.filledTonalButtonColors(
                    containerColor = btnBg,
                    contentColor = btnFg
                ),
                border = if (!isCurrentConnected && !colors.isDark) androidx.compose.foundation.BorderStroke(1.dp, Color(0xFFCBD5E1)) else null,
                shape = RoundedCornerShape(10.dp),
                modifier = Modifier.height(36.dp)
            ) {
                Text(
                    text = if (isCurrentConnected) "Active" else strings.btnConnect,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.SemiBold
                )
            }
        }
    }
}
