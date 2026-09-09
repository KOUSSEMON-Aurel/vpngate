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
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.material.icons.filled.Check
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
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
        else -> if (colors.isDark) colors.border else Color(0xFFCBD5E1)
    }

    val isDead = server.isReachable == false
    val (dotColor, latencyText) = when {
        server.isWarp -> colors.statusConnected to "15 ms"
        server.isReachable == true -> {
            val ms = server.verifiedLatency ?: server.effectivePing
            val color = when {
                ms <= 100 -> colors.statusConnected
                ms <= 260 -> Amber500
                else -> Color(0xFFD97706)
            }
            color to "$ms ms"
        }
        server.isReachable == false -> {
            Rose500 to "Hors ligne"
        }
        else -> {
            if (server.ping <= 0) {
                colors.textMuted to "—"
            } else {
                val ms = server.effectivePing
                val color = when {
                    ms <= 80 -> colors.statusConnected
                    ms <= 200 -> Amber500
                    else -> Color(0xFFD97706)
                }
                color to "${server.ping} ms"
            }
        }
    }

    Box(
        modifier = modifier
            .fillMaxWidth()
            .shadow(
                elevation = if (colors.isDark) 0.dp else 2.dp,
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
                        .size(40.dp)
                        .clip(RoundedCornerShape(10.dp))
                        .background(if (colors.isDark) colors.surfaceVariant else Color(0xFFE0F2FE))
                        .border(1.dp, if (colors.isDark) colors.border else Color(0xFFBAE6FD), RoundedCornerShape(10.dp)),
                    contentAlignment = Alignment.Center
                ) {
                    Text(
                        text = server.countryBadge,
                        color = if (colors.isDark) colors.accentSecondary else Color(0xFF0369A1),
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
                            fontWeight = FontWeight.Bold,
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
                        fontWeight = FontWeight.Medium,
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
                                .background(dotColor)
                        )
                        Spacer(modifier = Modifier.width(4.dp))
                        Text(
                            text = latencyText,
                            color = dotColor,
                            fontSize = 11.sp,
                            fontWeight = FontWeight.SemiBold
                        )

                        Spacer(modifier = Modifier.width(10.dp))

                        Box(
                            modifier = Modifier
                                .clip(RoundedCornerShape(4.dp))
                                .background(if (server.isWarp) (if (colors.isDark) colors.accentSecondary.copy(alpha = 0.15f) else Color(0xFFE0F2FE)) else (if (colors.isDark) colors.surfaceVariant else Color(0xFFF1F5F9)))
                                .border(1.dp, if (server.isWarp) (if (colors.isDark) colors.accentSecondary.copy(alpha = 0.3f) else Color(0xFFBAE6FD)) else (if (colors.isDark) colors.border else Color(0xFFCBD5E1)), RoundedCornerShape(4.dp))
                                .padding(horizontal = 6.dp, vertical = 2.dp)
                        ) {
                            Text(
                                text = if (server.isWarp) "WireGuard" else "OpenVPN",
                                color = if (server.isWarp) (if (colors.isDark) colors.accentSecondary else Color(0xFF0369A1)) else (if (colors.isDark) colors.textSecondary else Color(0xFF334155)),
                                fontSize = 10.sp,
                                fontWeight = FontWeight.Bold
                            )
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.width(10.dp))

            // Connect button (High contrast, vibrant, crisp action CTA)
            val buttonContainerColor = when {
                isCurrentConnected -> if (colors.isDark) Color(0xFF065F46) else Color(0xFF047857)
                isDead -> if (colors.isDark) Color(0xFF27272A) else Color(0xFFF1F5F9)
                else -> if (colors.isDark) Color(0xFF182E26) else Color(0xFF059669)
            }
            val buttonContentColor = when {
                isCurrentConnected -> Color.White
                isDead -> if (colors.isDark) Color(0xFF71717A) else Color(0xFF64748B)
                else -> if (colors.isDark) Color(0xFF34D399) else Color.White
            }
            val buttonBorder = when {
                isCurrentConnected -> null
                isDead -> BorderStroke(1.dp, if (colors.isDark) Color(0xFF3F3F46) else Color(0xFFCBD5E1))
                colors.isDark -> BorderStroke(1.dp, Color(0xFF10B981).copy(alpha = 0.5f))
                else -> BorderStroke(1.dp, Color(0xFF047857))
            }

            Button(
                onClick = onConnect,
                colors = ButtonDefaults.buttonColors(
                    containerColor = buttonContainerColor,
                    contentColor = buttonContentColor
                ),
                border = buttonBorder,
                elevation = ButtonDefaults.buttonElevation(
                    defaultElevation = if (colors.isDark || isDead) 0.dp else 1.5.dp,
                    pressedElevation = 0.5.dp
                ),
                shape = RoundedCornerShape(10.dp),
                contentPadding = PaddingValues(horizontal = 14.dp, vertical = 0.dp),
                modifier = Modifier.height(36.dp)
            ) {
                if (isCurrentConnected) {
                    Icon(
                        imageVector = Icons.Default.Check,
                        contentDescription = null,
                        tint = Color.White,
                        modifier = Modifier.size(14.dp)
                    )
                    Spacer(modifier = Modifier.width(4.dp))
                    Text(
                        text = "Actif",
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Bold,
                        color = Color.White
                    )
                } else if (isDead) {
                    Text(
                        text = "Hors ligne",
                        fontSize = 11.sp,
                        fontWeight = FontWeight.Medium,
                        color = buttonContentColor
                    )
                } else {
                    Text(
                        text = strings.btnConnect,
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Bold,
                        color = buttonContentColor
                    )
                }
            }
        }
    }
}
