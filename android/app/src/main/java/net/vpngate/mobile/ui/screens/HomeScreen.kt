package net.vpngate.mobile.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Shield
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import net.vpngate.mobile.data.model.ConnectionStatus
import net.vpngate.mobile.ui.components.HeroConnectButton
import net.vpngate.mobile.ui.components.StatsBar
import net.vpngate.mobile.ui.components.VpnDisclosureDialog
import net.vpngate.mobile.ui.components.map.WorldMapCard
import net.vpngate.mobile.ui.theme.AppTheme
import net.vpngate.mobile.ui.viewmodel.VpnViewModel

@Composable
fun HomeScreen(
    viewModel: VpnViewModel,
    onNavigateToServers: () -> Unit,
    onRequireVpnPermission: () -> Unit,
    modifier: Modifier = Modifier
) {
    val context = LocalContext.current
    val colors = AppTheme.colors
    val strings = AppTheme.strings

    val connectionState by viewModel.connectionState.collectAsState()
    val selectedServer by viewModel.selectedServer.collectAsState()
    val servers by viewModel.servers.collectAsState()
    val isLoading by viewModel.isLoading.collectAsState()
    val vpnDisclosureAccepted by viewModel.vpnDisclosureAccepted.collectAsState()

    var showDisclosureDialog by remember { mutableStateOf(false) }

    if (showDisclosureDialog) {
        VpnDisclosureDialog(
            onAccept = {
                showDisclosureDialog = false
                viewModel.setVpnDisclosureAccepted(true)
                viewModel.toggleConnection(context, onRequireVpnPermission)
            },
            onDismiss = {
                showDisclosureDialog = false
            }
        )
    }

    val currentServer = connectionState.connectedServer ?: selectedServer
    val status = connectionState.status

    val statusText = when (status) {
        ConnectionStatus.CONNECTED -> strings.statusConnected
        ConnectionStatus.CONNECTING -> strings.statusConnecting
        ConnectionStatus.DISCONNECTING -> strings.statusDisconnecting
        ConnectionStatus.ERROR -> strings.statusFailed
        ConnectionStatus.DISCONNECTED -> strings.statusReady
    }

    val statusColor = when (status) {
        ConnectionStatus.CONNECTED -> colors.statusConnected
        ConnectionStatus.CONNECTING -> colors.statusConnecting
        ConnectionStatus.ERROR -> colors.statusError
        else -> colors.textMuted
    }

    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        modifier = modifier
            .fillMaxSize()
            .background(colors.background)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 20.dp)
            .padding(top = 12.dp, bottom = 24.dp)
    ) {
        // Top Header
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween,
            modifier = Modifier.fillMaxWidth()
        ) {
            Text(
                text = "OPENRELAY",
                color = colors.textPrimary,
                fontWeight = FontWeight.Bold,
                fontSize = 18.sp,
                letterSpacing = 1.sp
            )

            IconButton(
                onClick = { viewModel.loadServers(forceRefresh = true) },
                enabled = !isLoading
            ) {
                if (isLoading) {
                    CircularProgressIndicator(
                        strokeWidth = 2.dp,
                        color = colors.statusConnected,
                        modifier = Modifier.size(20.dp)
                    )
                } else {
                    Icon(
                        imageVector = Icons.Default.Refresh,
                        contentDescription = "Refresh Servers",
                        tint = colors.textMuted
                    )
                }
            }
        }

        Spacer(modifier = Modifier.height(10.dp))

        // World Map Gateway Card with Glowing Points Lumineux & Flight Arc
        WorldMapCard(
            servers = servers,
            selectedServer = selectedServer,
            connectedServer = connectionState.connectedServer,
            status = status,
            onSelectServer = { server ->
                viewModel.selectServer(server)
            }
        )

        Spacer(modifier = Modifier.height(14.dp))

        // Hero Button
        HeroConnectButton(
            status = status,
            onClick = {
                if (status == ConnectionStatus.CONNECTED) {
                    viewModel.toggleConnection(context, onRequireVpnPermission)
                } else if (!vpnDisclosureAccepted) {
                    showDisclosureDialog = true
                } else {
                    viewModel.toggleConnection(context, onRequireVpnPermission)
                }
            }
        )

        Spacer(modifier = Modifier.height(10.dp))

        // Status pill
        val statusPillBg = when (status) {
            ConnectionStatus.CONNECTED -> if (colors.isDark) Color(0xFF064E3B).copy(alpha = 0.4f) else Color(0xFFD1FAE5)
            ConnectionStatus.CONNECTING -> if (colors.isDark) Color(0xFF164E63).copy(alpha = 0.4f) else Color(0xFFE0F2FE)
            ConnectionStatus.ERROR -> if (colors.isDark) Color(0xFF4C0519).copy(alpha = 0.4f) else Color(0xFFFFE4E6)
            else -> colors.surface
        }
        val statusPillBorder = when (status) {
            ConnectionStatus.CONNECTED -> colors.statusConnected
            ConnectionStatus.CONNECTING -> colors.statusConnecting
            ConnectionStatus.ERROR -> colors.statusError
            else -> colors.border
        }

        Box(
            modifier = Modifier
                .shadow(
                    elevation = if (colors.isDark) 0.dp else 2.dp,
                    shape = RoundedCornerShape(20.dp),
                    ambientColor = colors.cardShadowColor,
                    spotColor = colors.cardShadowColor
                )
                .clip(RoundedCornerShape(20.dp))
                .background(statusPillBg)
                .border(1.dp, statusPillBorder, RoundedCornerShape(20.dp))
                .padding(horizontal = 16.dp, vertical = 7.dp)
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Box(
                    modifier = Modifier
                        .size(8.dp)
                        .clip(CircleShape)
                        .background(statusColor)
                )
                Spacer(modifier = Modifier.width(8.dp))
                Text(
                    text = statusText,
                    color = statusColor,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Bold,
                    letterSpacing = 0.5.sp
                )
            }
        }

        Spacer(modifier = Modifier.height(14.dp))

        // Active / Selected Location Card
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .shadow(
                    elevation = if (colors.isDark) 0.dp else 4.dp,
                    shape = RoundedCornerShape(16.dp),
                    ambientColor = colors.cardShadowColor,
                    spotColor = colors.cardShadowColor
                )
                .clip(RoundedCornerShape(16.dp))
                .background(colors.surface)
                .border(1.dp, colors.border, RoundedCornerShape(16.dp))
                .clickable(onClick = onNavigateToServers)
                .padding(14.dp)
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.SpaceBetween,
                modifier = Modifier.fillMaxWidth()
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    modifier = Modifier.weight(1f)
                ) {
                    Box(
                        modifier = Modifier
                            .size(42.dp)
                            .clip(RoundedCornerShape(10.dp))
                            .background(if (colors.isDark) colors.surfaceVariant else Color(0xFFE0F2FE))
                            .border(1.dp, if (colors.isDark) colors.border else Color(0xFFBAE6FD), RoundedCornerShape(10.dp)),
                        contentAlignment = Alignment.Center
                    ) {
                        Text(
                            text = currentServer?.countryBadge ?: "VPN",
                            color = colors.accentSecondary,
                            fontSize = 13.sp,
                            fontWeight = FontWeight.Bold
                        )
                    }
                    Spacer(modifier = Modifier.width(12.dp))

                    Column(modifier = Modifier.weight(1f)) {
                        Text(
                            text = if (status == ConnectionStatus.CONNECTED) strings.connectedLocation else strings.selectedLocation,
                            color = colors.textSecondary,
                            fontSize = 11.sp,
                            maxLines = 1
                        )
                        Text(
                            text = currentServer?.countryLong ?: strings.selectGateway,
                            color = colors.textPrimary,
                            fontSize = 15.sp,
                            fontWeight = FontWeight.SemiBold,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis
                        )
                        Spacer(modifier = Modifier.height(2.dp))
                        if (currentServer != null) {
                            val protoLabel = if (currentServer.isWarp) "WireGuard" else "OpenVPN"
                            val ip = if (status == ConnectionStatus.CONNECTED && !connectionState.detectedPublicIp.isNullOrBlank()) {
                                connectionState.detectedPublicIp
                            } else {
                                currentServer.ip
                            }
                            Text(
                                text = "$ip • ${currentServer.ping} ms • $protoLabel",
                                color = if (status == ConnectionStatus.CONNECTED) colors.statusConnected else colors.textSecondary,
                                fontSize = 12.sp,
                                maxLines = 1,
                                overflow = TextOverflow.Ellipsis
                            )
                        }
                    }
                }

                Spacer(modifier = Modifier.width(8.dp))

                Icon(
                    imageVector = Icons.Default.ChevronRight,
                    contentDescription = "Change Server",
                    tint = colors.textMuted,
                    modifier = Modifier.size(24.dp)
                )
            }
        }

        if (status == ConnectionStatus.ERROR && connectionState.errorMessage != null) {
            Spacer(modifier = Modifier.height(12.dp))
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .clip(RoundedCornerShape(12.dp))
                    .background(if (colors.isDark) Color(0xFF271317) else Color(0xFFFFE4E6))
                    .border(1.dp, if (colors.isDark) Color(0xFF881337) else Color(0xFFFDA4AF), RoundedCornerShape(12.dp))
                    .padding(horizontal = 14.dp, vertical = 10.dp)
            ) {
                Text(
                    text = connectionState.errorMessage ?: "Connection error",
                    color = colors.statusError,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Medium
                )
            }
        } 
        
        Spacer(modifier = Modifier.height(14.dp))

        // Stats Dashboard
        StatsBar(
            status = status,
            durationSeconds = connectionState.durationSeconds,
            bytesIn = connectionState.bytesIn,
            bytesOut = connectionState.bytesOut
        )
    }
}
