package net.vpngate.mobile.ui.viewmodel

import android.content.Context
import android.content.Intent
import androidx.core.content.ContextCompat
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import net.vpngate.mobile.data.model.ConnectionStatus
import net.vpngate.mobile.data.model.VpnConnectionState
import net.vpngate.mobile.data.model.VpnServer
import net.vpngate.mobile.data.repository.VpnGateRepository
import net.vpngate.mobile.service.VpnGatewayService

enum class SortMode {
    PING,
    SPEED,
    SCORE
}

class VpnViewModel(
    private val repository: VpnGateRepository = VpnGateRepository()
) : ViewModel() {

    val connectionState: StateFlow<VpnConnectionState> = VpnGatewayService.connectionState

    private val _servers = MutableStateFlow<List<VpnServer>>(emptyList())
    val servers = _servers.asStateFlow()

    private val _selectedServer = MutableStateFlow<VpnServer?>(null)
    val selectedServer = _selectedServer.asStateFlow()

    private val _isLoading = MutableStateFlow(false)
    val isLoading = _isLoading.asStateFlow()

    private val _error = MutableStateFlow<String?>(null)
    val error = _error.asStateFlow()

    private val _searchQuery = MutableStateFlow("")
    val searchQuery = _searchQuery.asStateFlow()

    private val _selectedCountry = MutableStateFlow<String?>(null)
    val selectedCountry = _selectedCountry.asStateFlow()

    private val _sortMode = MutableStateFlow(SortMode.PING)
    val sortMode = _sortMode.asStateFlow()

    val availableCountries: StateFlow<List<String>> = _servers.combine(_searchQuery) { list, _ ->
        list.map { it.countryShort }.distinct().filter { it.isNotBlank() }.sorted()
    }.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), emptyList())

    val filteredServers: StateFlow<List<VpnServer>> = combine(
        _servers,
        _searchQuery,
        _selectedCountry,
        _sortMode
    ) { list, query, country, sort ->
        var filtered = list

        if (country != null) {
            filtered = filtered.filter { it.countryShort.equals(country, ignoreCase = true) }
        }

        if (query.isNotBlank()) {
            val q = query.trim().lowercase()
            filtered = filtered.filter {
                it.countryLong.lowercase().contains(q) ||
                it.ip.contains(q) ||
                it.countryShort.lowercase().contains(q) ||
                it.hostName.lowercase().contains(q)
            }
        }

        when (sort) {
            SortMode.PING -> filtered.sortedBy { it.ping.takeIf { p -> p > 0 } ?: 999L }
            SortMode.SPEED -> filtered.sortedByDescending { it.speed }
            SortMode.SCORE -> filtered.sortedByDescending { it.score }
        }
    }.stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), emptyList())

    init {
        loadServers()
    }

    fun loadServers(forceRefresh: Boolean = false) {
        viewModelScope.launch {
            _isLoading.value = true
            _error.value = null
            try {
                val fetched = repository.getServers(forceRefresh)
                _servers.value = fetched
                if (_selectedServer.value == null && fetched.isNotEmpty()) {
                    _selectedServer.value = repository.findBestServer(fetched) ?: fetched.first()
                }

                // Ping top servers asynchronously in background
                if (fetched.isNotEmpty()) {
                    val pinged = repository.pingServers(fetched, limit = 15)
                    _servers.value = pinged
                    if (_selectedServer.value != null) {
                        _selectedServer.value = pinged.find { it.ip == _selectedServer.value?.ip }
                    }
                }
            } catch (e: Exception) {
                _error.value = "Failed to load servers: ${e.message}"
            } finally {
                _isLoading.value = false
            }
        }
    }

    fun selectServer(server: VpnServer) {
        _selectedServer.value = server
    }

    fun updateSearchQuery(query: String) {
        _searchQuery.value = query
    }

    fun setCountryFilter(country: String?) {
        _selectedCountry.value = country
    }

    fun setSortMode(mode: SortMode) {
        _sortMode.value = mode
    }

    fun toggleConnection(context: Context, onRequirePermission: () -> Unit) {
        if (connectionState.value.status == ConnectionStatus.CONNECTED) {
            disconnect(context)
        } else {
            val target = _selectedServer.value ?: repository.findBestServer(_servers.value)
            if (target != null) {
                connect(context, target, onRequirePermission)
            }
        }
    }

    fun connect(context: Context, server: VpnServer, onRequirePermission: () -> Unit) {
        _selectedServer.value = server

        val intent = Intent(context, VpnGatewayService::class.java).apply {
            action = VpnGatewayService.ACTION_CONNECT
            putExtra(VpnGatewayService.EXTRA_IP, server.ip)
            putExtra(VpnGatewayService.EXTRA_HOSTNAME, server.hostName)
            putExtra(VpnGatewayService.EXTRA_COUNTRY, server.countryLong)
            putExtra(VpnGatewayService.EXTRA_COUNTRY_SHORT, server.countryShort)
            putExtra(VpnGatewayService.EXTRA_PING, server.ping)
            putExtra(VpnGatewayService.EXTRA_SPEED, server.speed)
            putExtra(VpnGatewayService.EXTRA_CONFIG, server.openVpnConfigDataBase64)
        }

        ContextCompat.startForegroundService(context, intent)
    }

    fun disconnect(context: Context) {
        val intent = Intent(context, VpnGatewayService::class.java).apply {
            action = VpnGatewayService.ACTION_DISCONNECT
        }
        context.startService(intent)
    }
}
