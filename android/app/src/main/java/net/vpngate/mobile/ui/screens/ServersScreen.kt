package net.vpngate.mobile.ui.screens

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Clear
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import net.vpngate.mobile.data.model.ConnectionStatus
import net.vpngate.mobile.ui.components.FilterChipGroup
import net.vpngate.mobile.ui.components.ServerCard
import net.vpngate.mobile.ui.theme.Emerald400
import net.vpngate.mobile.ui.theme.Emerald500
import net.vpngate.mobile.ui.theme.Zinc100
import net.vpngate.mobile.ui.theme.Zinc400
import net.vpngate.mobile.ui.theme.Zinc700
import net.vpngate.mobile.ui.theme.Zinc800
import net.vpngate.mobile.ui.theme.Zinc900
import net.vpngate.mobile.ui.theme.Zinc950
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
            .background(Zinc950)
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
                    tint = Zinc100
                )
            }
            Text(
                text = "Relay Servers (${servers.size})",
                color = Zinc100,
                fontSize = 18.sp,
                fontWeight = FontWeight.Bold,
                modifier = Modifier.weight(1f)
            )
            if (isLoading) {
                CircularProgressIndicator(
                    strokeWidth = 2.dp,
                    color = Emerald500,
                    modifier = Modifier.size(20.dp)
                )
            }
        }

        Spacer(modifier = Modifier.height(8.dp))

        // Search Bar
        OutlinedTextField(
            value = searchQuery,
            onValueChange = { viewModel.updateSearchQuery(it) },
            placeholder = { Text("Search by country or IP…", color = Zinc400, fontSize = 13.sp) },
            leadingIcon = {
                Icon(
                    imageVector = Icons.Default.Search,
                    contentDescription = null,
                    tint = Zinc400,
                    modifier = Modifier.size(18.dp)
                )
            },
            trailingIcon = {
                if (searchQuery.isNotEmpty()) {
                    IconButton(onClick = { viewModel.updateSearchQuery("") }) {
                        Icon(
                            imageVector = Icons.Default.Clear,
                            contentDescription = "Clear",
                            tint = Zinc400,
                            modifier = Modifier.size(16.dp)
                        )
                    }
                }
            },
            singleLine = true,
            shape = RoundedCornerShape(12.dp),
            colors = OutlinedTextFieldDefaults.colors(
                focusedContainerColor = Zinc900,
                unfocusedContainerColor = Zinc900,
                focusedBorderColor = Emerald500,
                unfocusedBorderColor = Zinc800,
                focusedTextColor = Zinc100,
                unfocusedTextColor = Zinc100
            ),
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp)
                .height(50.dp)
        )

        Spacer(modifier = Modifier.height(10.dp))

        // Country Filter Chips
        FilterChipGroup(
            countries = countries,
            selectedCountry = selectedCountry,
            onSelectCountry = { viewModel.setCountryFilter(it) },
            modifier = Modifier.padding(horizontal = 12.dp)
        )

        Spacer(modifier = Modifier.height(6.dp))

        // Sort Row
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.End,
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 16.dp)
        ) {
            Text("Sort: ", color = Zinc400, fontSize = 11.sp)
            SortTextButton("Ping", sortMode == SortMode.PING) { viewModel.setSortMode(SortMode.PING) }
            SortTextButton("Speed", sortMode == SortMode.SPEED) { viewModel.setSortMode(SortMode.SPEED) }
            SortTextButton("Score", sortMode == SortMode.SCORE) { viewModel.setSortMode(SortMode.SCORE) }
        }

        Spacer(modifier = Modifier.height(6.dp))

        // Server List
        if (servers.isEmpty() && !isLoading) {
            Box(
                contentAlignment = Alignment.Center,
                modifier = Modifier.fillMaxSize()
            ) {
                Text(
                    text = "No VPN servers found matching filters",
                    color = Zinc400,
                    fontSize = 14.sp
                )
            }
        } else {
            LazyColumn(
                contentPadding = PaddingValues(horizontal = 16.dp, vertical = 8.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp),
                modifier = Modifier.fillMaxSize()
            ) {
                items(servers, key = { it.ip }) { server ->
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
    TextButton(
        onClick = onClick,
        contentPadding = PaddingValues(horizontal = 6.dp, vertical = 2.dp)
    ) {
        Text(
            text = label,
            fontSize = 11.sp,
            fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Normal,
            color = if (isSelected) Emerald400 else Zinc400
        )
    }
}
