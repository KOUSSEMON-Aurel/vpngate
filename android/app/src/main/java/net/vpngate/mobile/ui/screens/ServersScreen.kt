package net.vpngate.mobile.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Clear
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
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
import net.vpngate.mobile.ui.components.FilterChipGroup
import net.vpngate.mobile.ui.components.ServerCard
import net.vpngate.mobile.ui.theme.AppTheme
import net.vpngate.mobile.ui.viewmodel.SortMode
import net.vpngate.mobile.ui.viewmodel.VpnViewModel

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ServersScreen(
    viewModel: VpnViewModel,
    onNavigateBack: () -> Unit,
    onRequireVpnPermission: () -> Unit,
    modifier: Modifier = Modifier
) {
    val context = LocalContext.current
    val colors = AppTheme.colors
    val strings = AppTheme.strings

    val servers by viewModel.filteredServers.collectAsState()
    val countries by viewModel.availableCountries.collectAsState()
    val selectedCountry by viewModel.selectedCountry.collectAsState()
    val searchQuery by viewModel.searchQuery.collectAsState()
    val sortMode by viewModel.sortMode.collectAsState()
    val selectedServer by viewModel.selectedServer.collectAsState()
    val connectionState by viewModel.connectionState.collectAsState()
    val isLoading by viewModel.isLoading.collectAsState()

    Column(
        modifier = modifier
            .fillMaxSize()
            .background(colors.background)
            .imePadding()
            .padding(top = 8.dp)
    ) {
        // Header
        Row(
            verticalAlignment = Alignment.CenterVertically,
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp)
        ) {
            IconButton(onClick = onNavigateBack) {
                Icon(
                    imageVector = Icons.AutoMirrored.Filled.ArrowBack,
                    contentDescription = "Back",
                    tint = colors.textPrimary
                )
            }
            Text(
                text = "${strings.relaysTitle} (${servers.size})",
                color = colors.textPrimary,
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                modifier = Modifier.weight(1f)
            )
            IconButton(onClick = { viewModel.loadServers(forceRefresh = true) }) {
                if (isLoading) {
                    CircularProgressIndicator(
                        strokeWidth = 2.dp,
                        color = colors.statusConnected,
                        modifier = Modifier.size(20.dp)
                    )
                } else {
                    Icon(
                        imageVector = Icons.Default.Refresh,
                        contentDescription = "Refresh Live Relays",
                        tint = colors.textPrimary
                    )
                }
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        // Protocol Filter Tabs: ALL, WARP, OPENVPN
        val protocolFilter by viewModel.protocolFilter.collectAsState()
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .horizontalScroll(rememberScrollState())
                .padding(horizontal = 16.dp),
            horizontalArrangement = Arrangement.spacedBy(8.dp)
        ) {
            net.vpngate.mobile.ui.viewmodel.ProtocolFilter.entries.forEach { filter ->
                val isSelected = protocolFilter == filter
                val label = when (filter) {
                    net.vpngate.mobile.ui.viewmodel.ProtocolFilter.ALL -> strings.filterAll
                    net.vpngate.mobile.ui.viewmodel.ProtocolFilter.WARP -> strings.filterWarp
                    net.vpngate.mobile.ui.viewmodel.ProtocolFilter.OPENVPN -> strings.filterOpenVpn
                }
                Box(
                    modifier = Modifier
                        .clip(RoundedCornerShape(8.dp))
                        .background(if (isSelected) colors.accentPrimary else colors.surface)
                        .border(1.dp, if (isSelected) colors.accentPrimary else colors.border, RoundedCornerShape(8.dp))
                        .clickable { viewModel.setProtocolFilter(filter) }
                        .padding(horizontal = 12.dp, vertical = 7.dp)
                ) {
                    Text(
                        text = label,
                        color = if (isSelected) (if (colors.isDark) Color(0xFF09090B) else Color.White) else colors.textPrimary,
                        fontSize = 12.sp,
                        fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Medium
                    )
                }
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        // Search Bar
        OutlinedTextField(
            value = searchQuery,
            onValueChange = { viewModel.updateSearchQuery(it) },
            placeholder = { Text(strings.searchPlaceholder, color = colors.textMuted, fontSize = 13.sp) },
            leadingIcon = {
                Icon(
                    imageVector = Icons.Default.Search,
                    contentDescription = null,
                    tint = colors.textMuted,
                    modifier = Modifier.size(18.dp)
                )
            },
            trailingIcon = {
                if (searchQuery.isNotEmpty()) {
                    IconButton(onClick = { viewModel.updateSearchQuery("") }) {
                        Icon(
                            imageVector = Icons.Default.Clear,
                            contentDescription = "Clear",
                            tint = colors.textMuted,
                            modifier = Modifier.size(18.dp)
                        )
                    }
                }
            },
            colors = OutlinedTextFieldDefaults.colors(
                focusedContainerColor = colors.surface,
                unfocusedContainerColor = colors.surface,
                focusedBorderColor = colors.accentPrimary,
                unfocusedBorderColor = colors.border,
                focusedTextColor = colors.textPrimary,
                unfocusedTextColor = colors.textPrimary,
                cursorColor = colors.accentPrimary
            ),
            shape = RoundedCornerShape(12.dp),
            singleLine = true,
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp)
                .height(50.dp)
        )

        Spacer(modifier = Modifier.height(8.dp))

        // Country Filter Chips
        if (countries.isNotEmpty()) {
            FilterChipGroup(
                countries = countries,
                selectedCountry = selectedCountry,
                onSelectCountry = { viewModel.selectCountry(it) },
                modifier = Modifier.padding(horizontal = 12.dp)
            )
            Spacer(modifier = Modifier.height(4.dp))
        }

        // Sort Bar
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.End,
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 2.dp)
        ) {
            Text(
                text = strings.sortLabel,
                color = colors.textSecondary,
                fontSize = 11.sp,
                modifier = Modifier.padding(end = 6.dp)
            )
            SortTextButton(strings.sortPing, sortMode == SortMode.PING) {
                viewModel.setSortMode(SortMode.PING)
            }
            Spacer(modifier = Modifier.width(4.dp))
            SortTextButton(strings.sortSpeed, sortMode == SortMode.SPEED) {
                viewModel.setSortMode(SortMode.SPEED)
            }
            Spacer(modifier = Modifier.width(4.dp))
            SortTextButton(strings.sortScore, sortMode == SortMode.SCORE) {
                viewModel.setSortMode(SortMode.SCORE)
            }
        }

        Spacer(modifier = Modifier.height(4.dp))

        // Server List
        if (servers.isEmpty()) {
            Box(
                contentAlignment = Alignment.Center,
                modifier = Modifier.fillMaxSize()
            ) {
                Text(
                    text = strings.noServersFound,
                    color = colors.textMuted,
                    fontSize = 14.sp
                )
            }
        } else {
            LazyColumn(
                contentPadding = PaddingValues(start = 16.dp, top = 8.dp, end = 16.dp, bottom = 24.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp),
                modifier = Modifier.fillMaxSize()
            ) {
                items(servers, key = { "${it.source}_${it.hostName}_${it.ip}_${it.remotePort}_${it.protocol}" }) { server ->
                    val isCurrentConnected =
                        connectionState.status == ConnectionStatus.CONNECTED &&
                        connectionState.connectedServer?.ip == server.ip

                    ServerCard(
                        server = server,
                        isSelected = selectedServer?.ip == server.ip,
                        isCurrentConnected = isCurrentConnected,
                        onSelect = {
                            viewModel.selectServer(server)
                        },
                        onConnect = {
                            viewModel.connect(context, server, onRequireVpnPermission)
                            onNavigateBack()
                        }
                    )
                }
            }
        }
    }
}

@Composable
private fun SortTextButton(
    label: String,
    isSelected: Boolean,
    onClick: () -> Unit
) {
    val colors = AppTheme.colors
    TextButton(
        onClick = onClick,
        contentPadding = PaddingValues(horizontal = 6.dp, vertical = 2.dp)
    ) {
        Text(
            text = label,
            fontSize = 11.sp,
            color = if (isSelected) colors.accentPrimary else colors.textSecondary,
            fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Normal
        )
    }
}
