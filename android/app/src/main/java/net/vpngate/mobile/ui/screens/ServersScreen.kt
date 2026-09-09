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
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.BorderStroke
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.Clear
import androidx.compose.material.icons.filled.FilterList
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.SwapVert
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
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

        Spacer(modifier = Modifier.height(6.dp))

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

        // Single-Row Controls Bar: Filtre (Left), Actifs Toggle (Center), Tri (Right)
        val relayFilter by viewModel.relayFilter.collectAsState()
        val onlyActive by viewModel.onlyActive.collectAsState()
        val sortMode by viewModel.sortMode.collectAsState()
        var showFilterMenu by remember { mutableStateOf(false) }
        var showSortMenu by remember { mutableStateOf(false) }

        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp, vertical = 6.dp)
        ) {
            // 1. FILTER DROPDOWN
            Box(modifier = Modifier.weight(1f)) {
                val filterLabel = when (relayFilter) {
                    net.vpngate.mobile.ui.viewmodel.RelayFilterMode.ALL -> "Tous"
                    net.vpngate.mobile.ui.viewmodel.RelayFilterMode.WARP -> "WireGuard"
                    net.vpngate.mobile.ui.viewmodel.RelayFilterMode.VPNBOOK -> "VPNBook"
                    net.vpngate.mobile.ui.viewmodel.RelayFilterMode.VPNGATE -> "VPNGate"
                }
                val isFilterActive = relayFilter != net.vpngate.mobile.ui.viewmodel.RelayFilterMode.ALL

                Surface(
                    onClick = { showFilterMenu = true },
                    shape = RoundedCornerShape(10.dp),
                    color = colors.surface,
                    border = BorderStroke(
                        width = 1.dp,
                        color = if (colors.isDark) colors.border else Color(0xFFCBD5E1)
                    ),
                    shadowElevation = if (colors.isDark) 0.dp else 1.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.Center,
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 8.dp)
                    ) {
                        Icon(
                            imageVector = Icons.Default.FilterList,
                            contentDescription = "Filtre",
                            tint = colors.textSecondary,
                            modifier = Modifier.size(15.dp)
                        )
                        Spacer(modifier = Modifier.width(4.dp))
                        Text(
                            text = filterLabel,
                            color = colors.textPrimary,
                            fontSize = 12.sp,
                            fontWeight = FontWeight.Bold,
                            maxLines = 1
                        )
                        if (isFilterActive) {
                            Spacer(modifier = Modifier.width(4.dp))
                            Box(
                                modifier = Modifier
                                    .size(5.dp)
                                    .clip(CircleShape)
                                    .background(colors.accentPrimary)
                            )
                        }
                        Spacer(modifier = Modifier.width(2.dp))
                        Icon(
                            imageVector = Icons.Default.KeyboardArrowDown,
                            contentDescription = null,
                            tint = colors.textMuted,
                            modifier = Modifier.size(15.dp)
                        )
                    }
                }

                DropdownMenu(
                    expanded = showFilterMenu,
                    onDismissRequest = { showFilterMenu = false },
                    shape = RoundedCornerShape(14.dp),
                    containerColor = colors.surface,
                    shadowElevation = if (colors.isDark) 0.dp else 8.dp,
                    border = BorderStroke(1.dp, if (colors.isDark) colors.border else Color(0xFFCBD5E1))
                ) {
                    val filterOptions = listOf(
                        net.vpngate.mobile.ui.viewmodel.RelayFilterMode.ALL to "Tous les relais",
                        net.vpngate.mobile.ui.viewmodel.RelayFilterMode.WARP to "WireGuard (WARP)",
                        net.vpngate.mobile.ui.viewmodel.RelayFilterMode.VPNBOOK to "VPNBook",
                        net.vpngate.mobile.ui.viewmodel.RelayFilterMode.VPNGATE to "VPNGate"
                    )

                    filterOptions.forEach { (mode, label) ->
                        val isSelected = relayFilter == mode
                        DropdownMenuItem(
                            text = {
                                Text(
                                    text = label,
                                    fontSize = 13.sp,
                                    fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Medium,
                                    color = if (isSelected) (if (colors.isDark) colors.statusConnected else Color(0xFF047857)) else colors.textPrimary
                                )
                            },
                            trailingIcon = if (isSelected) {
                                {
                                    Icon(
                                        imageVector = Icons.Default.Check,
                                        contentDescription = null,
                                        tint = if (colors.isDark) colors.statusConnected else Color(0xFF047857),
                                        modifier = Modifier.size(16.dp)
                                    )
                                }
                            } else null,
                            onClick = {
                                viewModel.setRelayFilter(mode)
                                showFilterMenu = false
                            }
                        )
                    }
                }
            }

            // 2. ACTIFS UNIQUEMENT TOGGLE (Direct on the row)
            Surface(
                onClick = { viewModel.toggleOnlyActive() },
                shape = RoundedCornerShape(10.dp),
                color = if (onlyActive) {
                    if (colors.isDark) Color(0xFF065F46) else Color(0xFF059669)
                } else colors.surface,
                border = BorderStroke(
                    width = 1.dp,
                    color = if (onlyActive) {
                        if (colors.isDark) Color(0xFF10B981) else Color(0xFF047857)
                    } else (if (colors.isDark) colors.border else Color(0xFFCBD5E1))
                ),
                shadowElevation = if (onlyActive) (if (colors.isDark) 0.dp else 2.dp) else (if (colors.isDark) 0.dp else 1.dp)
            ) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp)
                ) {
                    if (onlyActive) {
                        Icon(
                            imageVector = Icons.Default.Check,
                            contentDescription = null,
                            tint = Color.White,
                            modifier = Modifier.size(13.dp)
                        )
                        Spacer(modifier = Modifier.width(5.dp))
                    } else {
                        Box(
                            modifier = Modifier
                                .size(7.dp)
                                .clip(CircleShape)
                                .background(colors.textMuted)
                        )
                        Spacer(modifier = Modifier.width(6.dp))
                    }
                    Text(
                        text = "Actifs",
                        color = if (onlyActive) Color.White else colors.textPrimary,
                        fontSize = 12.sp,
                        fontWeight = FontWeight.Bold
                    )
                }
            }

            // 3. SORT DROPDOWN
            Box(modifier = Modifier.weight(1f)) {
                val sortLabel = when (sortMode) {
                    SortMode.PING -> strings.sortPing
                    SortMode.SPEED -> strings.sortSpeed
                    SortMode.SCORE -> strings.sortScore
                    SortMode.COUNTRY -> "Pays"
                }

                Surface(
                    onClick = { showSortMenu = true },
                    shape = RoundedCornerShape(10.dp),
                    color = colors.surface,
                    border = BorderStroke(
                        width = 1.dp,
                        color = if (colors.isDark) colors.border else Color(0xFFCBD5E1)
                    ),
                    shadowElevation = if (colors.isDark) 0.dp else 1.dp,
                    modifier = Modifier.fillMaxWidth()
                ) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.Center,
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 8.dp)
                    ) {
                        Icon(
                            imageVector = Icons.Default.SwapVert,
                            contentDescription = "Tri",
                            tint = colors.textSecondary,
                            modifier = Modifier.size(15.dp)
                        )
                        Spacer(modifier = Modifier.width(4.dp))
                        Text(
                            text = sortLabel,
                            color = colors.textPrimary,
                            fontSize = 12.sp,
                            fontWeight = FontWeight.Bold,
                            maxLines = 1
                        )
                        Spacer(modifier = Modifier.width(2.dp))
                        Icon(
                            imageVector = Icons.Default.KeyboardArrowDown,
                            contentDescription = null,
                            tint = colors.textMuted,
                            modifier = Modifier.size(15.dp)
                        )
                    }
                }

                DropdownMenu(
                    expanded = showSortMenu,
                    onDismissRequest = { showSortMenu = false },
                    shape = RoundedCornerShape(14.dp),
                    containerColor = colors.surface,
                    shadowElevation = if (colors.isDark) 0.dp else 8.dp,
                    border = BorderStroke(1.dp, if (colors.isDark) colors.border else Color(0xFFCBD5E1))
                ) {
                    val sortOptions = listOf(
                        SortMode.PING to "Latence minimale (Ping)",
                        SortMode.SPEED to "Débit maximal (Vitesse)",
                        SortMode.SCORE to "Score de stabilité",
                        SortMode.COUNTRY to "Pays (A à Z)"
                    )

                    sortOptions.forEach { (mode, label) ->
                        val isSelected = sortMode == mode
                        DropdownMenuItem(
                            text = {
                                Text(
                                    text = label,
                                    fontSize = 13.sp,
                                    fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Medium,
                                    color = if (isSelected) (if (colors.isDark) colors.statusConnected else Color(0xFF047857)) else colors.textPrimary
                                )
                            },
                            trailingIcon = if (isSelected) {
                                {
                                    Icon(
                                        imageVector = Icons.Default.Check,
                                        contentDescription = null,
                                        tint = if (colors.isDark) colors.statusConnected else Color(0xFF047857),
                                        modifier = Modifier.size(16.dp)
                                    )
                                }
                            } else null,
                            onClick = {
                                viewModel.setSortMode(mode)
                                showSortMenu = false
                            }
                        )
                    }
                }
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
                itemsIndexed(servers, key = { index, server -> "${server.source}_${server.hostName}_${server.ip}_${server.remotePort}_${server.protocol}_$index" }) { _, server ->
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
