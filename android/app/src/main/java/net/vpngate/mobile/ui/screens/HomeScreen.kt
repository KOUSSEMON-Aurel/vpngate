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
import androidx.compose.material.icons.filled.Public
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Shield
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import net.vpngate.mobile.data.model.ConnectionStatus
import net.vpngate.mobile.ui.components.HeroConnectButton
import net.vpngate.mobile.ui.components.StatsBar
import net.vpngate.mobile.ui.theme.Amber500
import net.vpngate.mobile.ui.theme.Cyan400
import net.vpngate.mobile.ui.theme.Emerald400
import net.vpngate.mobile.ui.theme.Emerald500
import net.vpngate.mobile.ui.theme.Rose500
import net.vpngate.mobile.ui.theme.Zinc100
import net.vpngate.mobile.ui.theme.Zinc400
import net.vpngate.mobile.ui.theme.Zinc700
import net.vpngate.mobile.ui.theme.Zinc800
import net.vpngate.mobile.ui.theme.Zinc900
import net.vpngate.mobile.ui.theme.Zinc950
import net.vpngate.mobile.ui.viewmodel.VpnViewModel

@Composable
fun HomeScreen(
    viewModel: VpnViewModel,
    onNavigateToServers: () -> Unit,
    onRequireVpnPermission: () -> Unit,
    modifier: Modifier = Modifier
) {
    val context = LocalContext.current
    val connectionState by viewModel.connectionState.collectAsState()
    val selectedServer by viewModel.selectedServer.collectAsState()
    val isLoading by viewModel.isLoading.collectAsState()

    val currentServer = connectionState.connectedServer ?: selectedServer
    val status = connectionState.status

    val statusText = when (status) {
        ConnectionStatus.CONNECTED -> "CONNECTED"
        ConnectionStatus.CONNECTING -> "CONNECTING…"
        ConnectionStatus.DISCONNECTING -> "DISCONNECTING…"
        ConnectionStatus.ERROR -> "CONNECTION FAILED"
        ConnectionStatus.DISCONNECTED -> "READY TO CONNECT"
    }

    val statusColor = when (status) {
        ConnectionStatus.CONNECTED -> Emerald400
        ConnectionStatus.CONNECTING -> Cyan400
        ConnectionStatus.ERROR -> Rose500
        else -> Zinc400
    }

    Column(
        horizontalAlignment = Alignment.CenterHorizontally,
        modifier = modifier
            .fillMaxSize()
            .background(Zinc950)
            .verticalScroll(rememberScrollState())
            .padding(horizontal = 20.dp, vertical = 16.dp)
    ) {
        // Top Header
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween,
            modifier = Modifier.fillMaxWidth()
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(
                    imageVector = Icons.Default.Shield,
                    contentDescription = null,
                    tint = if (status == ConnectionStatus.CONNECTED) Emerald500 else Zinc400,
                    modifier = Modifier.size(24.dp)
                )
                Spacer(modifier = Modifier.width(8.dp))
                Text(
                    text = "VPNGATE",
                    color = Zinc100,
                    fontWeight = FontWeight.Bold,
                    fontSize = 18.sp,
                    letterSpacing = 1.sp
                )
            }

            IconButton(
                onClick = { viewModel.loadServers(forceRefresh = true) },
                enabled = !isLoading
            ) {
                if (isLoading) {
                    CircularProgressIndicator(
                        strokeWidth = 2.dp,
                        color = Emerald500,
                        modifier = Modifier.size(20.dp)
                    )
                } else {
                    Icon(
                        imageVector = Icons.Default.Refresh,
                        contentDescription = "Refresh Servers",
                        tint = Zinc400
                    )
                }
            }
        }

        Spacer(modifier = Modifier.height(24.dp))

        // Hero Button
        HeroConnectButton(
            status = status,
            onClick = {
                viewModel.toggleConnection(context, onRequireVpnPermission)
            }
        )

        Spacer(modifier = Modifier.height(16.dp))

        // Status pill
        Box(
            modifier = Modifier
                .clip(RoundedCornerShape(20.dp))
                .background(Zinc900)
                .border(1.dp, Zinc800, RoundedCornerShape(20.dp))
                .padding(horizontal = 16.dp, vertical = 6.dp)
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
                    fontWeight = FontWeight.SemiBold,
                    letterSpacing = 0.5.sp
                )
            }
        }

        Spacer(modifier = Modifier.height(28.dp))

        // Active / Selected Location Card
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(16.dp))
                .background(Zinc900)
                .border(1.dp, Zinc800, RoundedCornerShape(16.dp))
                .clickable(onClick = onNavigateToServers)
                .padding(16.dp)
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
                            .background(Zinc800)
                            .border(1.dp, Zinc700, RoundedCornerShape(10.dp)),
                        contentAlignment = Alignment.Center
                    ) {
                        Text(
                            text = currentServer?.countryBadge ?: "VPN",
                            color = Color(0xFF38BDF8),
                            fontSize = 13.sp,
                            fontWeight = FontWeight.Bold
                        )
                    }
                    Spacer(modifier = Modifier.width(14.dp))

                    Column {
                        Text(
                            text = if (status == ConnectionStatus.CONNECTED) "Connected Location" else "Selected Location",
                            color = Zinc400,
                            fontSize = 11.sp
                        )
                        Text(
                            text = currentServer?.countryLong ?: "Select a Gateway",
                            color = Zinc100,
                            fontSize = 16.sp,
                            fontWeight = FontWeight.SemiBold
                        )
                        if (currentServer != null) {
                            val protoLabel = if (currentServer.isWarp) "WireGuard" else "OpenVPN"
                            val ipDisplay = if (status == ConnectionStatus.CONNECTED && !connectionState.detectedPublicIp.isNullOrBlank()) {
                                "Public IP: ${connectionState.detectedPublicIp}"
                            } else {
                                currentServer.ip
                            }
                            Text(
                                text = "$ipDisplay • ${currentServer.ping} ms • $protoLabel",
                                color = if (status == ConnectionStatus.CONNECTED) Emerald400 else Zinc400,
                                fontSize = 12.sp
                            )
                        }
                    }
                }

                Icon(
                    imageVector = Icons.Default.ChevronRight,
                    contentDescription = "Change Server",
                    tint = Zinc400,
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
                    .background(Color(0xFF271317))
                    .border(1.dp, Color(0xFF881337), RoundedCornerShape(12.dp))
                    .padding(horizontal = 14.dp, vertical = 10.dp)
            ) {
                Text(
                    text = connectionState.errorMessage ?: "Connection error",
                    color = Rose500,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Medium
                )
            }
        } 
        
        Spacer(modifier = Modifier.height(20.dp))

        // Stats Dashboard
        StatsBar(
            status = status,
            durationSeconds = connectionState.durationSeconds,
            bytesIn = connectionState.bytesIn,
            bytesOut = connectionState.bytesOut
        )
    }
}
