package net.vpngate.mobile.ui.viewmodel

import android.app.Application
import android.content.Context
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.FlowPreview
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.debounce
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import net.vpngate.mobile.data.model.ConnectionStatus
import net.vpngate.mobile.data.model.VpnConnectionState
import net.vpngate.mobile.data.model.VpnServer
import net.vpngate.mobile.data.prefs.AppLanguage
import net.vpngate.mobile.data.prefs.AppPreferences
import net.vpngate.mobile.data.prefs.ProtocolPreference
import net.vpngate.mobile.data.prefs.ThemeMode
import net.vpngate.mobile.data.repository.VpnGateRepository
import net.vpngate.mobile.service.UnifiedTunnelManager

enum class SortMode {
    PING,
    SPEED,
    SCORE
}

enum class ProtocolFilter {
    ALL,
    WARP,
    OPENVPN
}

class VpnViewModel @JvmOverloads constructor(
    application: Application,
    private val repository: VpnGateRepository = VpnGateRepository()
) : AndroidViewModel(application) {

    private val appPreferences = AppPreferences.getInstance(application)
    val themeMode: StateFlow<ThemeMode> = appPreferences.themeMode
    val language: StateFlow<AppLanguage> = appPreferences.language
    val protocolPreference: StateFlow<ProtocolPreference> = appPreferences.protocolPreference
    val dnsProtection: StateFlow<Boolean> = appPreferences.dnsProtection

    val connectionState: StateFlow<VpnConnectionState> = UnifiedTunnelManager.connectionState

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

    private val _protocolFilter = MutableStateFlow(ProtocolFilter.ALL)
    val protocolFilter = _protocolFilter.asStateFlow()

    private val _sortMode = MutableStateFlow(SortMode.PING)
    val sortMode = _sortMode.asStateFlow()

    val availableCountries: StateFlow<List<String>> = _servers.map { list ->
        list.filter { !it.isWarp }
            .map { it.countryShort }
            .distinct()
            .filter { it.isNotBlank() }
            .sorted()
    }.flowOn(Dispatchers.Default)
     .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), emptyList())

    @OptIn(FlowPreview::class)
    val filteredServers: StateFlow<List<VpnServer>> = combine(
        _servers,
        _searchQuery.debounce(150),
        _selectedCountry,
        _protocolFilter,
        _sortMode
    ) { list, query, country, proto, sort ->
        var filtered = list

        // Protocol filtering
        when (proto) {
            ProtocolFilter.ALL -> {}
            ProtocolFilter.WARP -> filtered = filtered.filter { it.isWarp }
            ProtocolFilter.OPENVPN -> filtered = filtered.filter { !it.isWarp }
        }

        if (country != null) {
            filtered = filtered.filter { it.countryShort.equals(country, ignoreCase = true) }
        }

        if (query.isNotBlank()) {
            val q = query.trim().lowercase()
            filtered = filtered.filter {
                it.countryLong.lowercase().contains(q) ||
                it.ip.contains(q) ||
                it.countryShort.lowercase().contains(q) ||
                it.hostName.lowercase().contains(q) ||
                it.operator.lowercase().contains(q)
            }
        }

        when (sort) {
            SortMode.PING -> filtered.sortedBy { if (it.isWarp) 0L else (it.ping.takeIf { p -> p > 0 } ?: 999L) }
            SortMode.SPEED -> filtered.sortedByDescending { it.speed }
            SortMode.SCORE -> filtered.sortedByDescending { it.score }
        }
    }.flowOn(Dispatchers.Default)
     .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), emptyList())

    private var isConnectingByUser = false
    private var failoverCount = 0
    private val failedIps = mutableSetOf<String>()
    private var isFetchingIp = false

    init {
        val initial = repository.getInitialServers(getApplication())
        if (initial.isNotEmpty()) {
            _servers.value = initial
            _selectedServer.value = repository.findBestServer(initial) ?: initial.first()
        }
        loadServers(forceRefresh = false)

        viewModelScope.launch {
            connectionState.collect { state ->
                when (state.status) {
                    ConnectionStatus.CONNECTED -> {
                        isConnectingByUser = false
                        failoverCount = 0
                        failedIps.clear()
                        // Only fetch IP once per connection — skip if already populated or in flight
                        if (state.detectedPublicIp.isNullOrBlank() && !isFetchingIp) {
                            isFetchingIp = true
                            viewModelScope.launch {
                                try {
                                    val publicIp = repository.getCurrentPublicIp()
                                    if (publicIp != null) {
                                        UnifiedTunnelManager.setDetectedPublicIp(publicIp)
                                    }
                                } catch (_: Exception) {
                                } finally {
                                    isFetchingIp = false
                                }
                            }
                        }
                    }
                    ConnectionStatus.DISCONNECTED -> {
                        isFetchingIp = false
                    }
                    ConnectionStatus.ERROR -> {
                        isFetchingIp = false
                        val failed = state.connectedServer
                        if (failed != null) {
                            failedIps.add(failed.ip)
                        }
                        if (isConnectingByUser && failoverCount < 3) {
                            failoverCount++
                            val candidates = _servers.value.filter { 
                                it.ip !in failedIps && it.ping < 9000L && it.openVpnConfigDataBase64.isNotBlank() 
                            }
                            val nextCandidate = repository.findBestServer(candidates)
                            if (nextCandidate != null) {
                                android.util.Log.i("VpnViewModel", "Automatic failover to ${nextCandidate.ip} (attempt $failoverCount/3)")
                                _selectedServer.value = nextCandidate
                                kotlinx.coroutines.delay(600)
                                UnifiedTunnelManager.startVpn(getApplication(), nextCandidate)
                                return@collect
                            }
                        }
                        isConnectingByUser = false
                    }
                    else -> {}
                }
            }
        }
    }

    fun setThemeMode(mode: ThemeMode) {
        appPreferences.setThemeMode(mode)
    }

    fun setLanguage(lang: AppLanguage) {
        appPreferences.setLanguage(lang)
    }

    fun setDnsProtection(enabled: Boolean) {
        appPreferences.setDnsProtection(enabled)
    }

    fun setProtocolPreference(pref: ProtocolPreference) {
        appPreferences.setProtocolPreference(pref)
    }

    fun clearCache() {
        viewModelScope.launch {
            _isLoading.value = true
            try {
                val cacheDir = getApplication<Application>().cacheDir
                java.io.File(cacheDir, "vpngate_cache.csv").delete()
                loadServers(forceRefresh = true)
            } finally {
                _isLoading.value = false
            }
        }
    }

    fun loadServers(forceRefresh: Boolean = false) {
        viewModelScope.launch {
            _isLoading.value = true
            _error.value = null
            try {
                val fetched = repository.getServers(getApplication(), forceRefresh)
                if (fetched.isNotEmpty()) {
                    _servers.value = fetched
                    if (connectionState.value.status != ConnectionStatus.CONNECTED &&
                        connectionState.value.status != ConnectionStatus.CONNECTING) {
                        _selectedServer.value = repository.findBestServer(fetched) ?: fetched.first()
                    }
                }

                // Ping top servers asynchronously in background
                if (fetched.isNotEmpty()) {
                    val pinged = repository.pingServers(fetched, limit = 20)
                    _servers.value = pinged
                    if (_selectedServer.value != null && !_selectedServer.value!!.isWarp) {
                        _selectedServer.value = pinged.find { it.ip == _selectedServer.value?.ip } ?: _selectedServer.value
                    }
                }
            } catch (e: Exception) {
                _error.value = "Failed to refresh servers: ${e.message}"
            } finally {
                _isLoading.value = false
            }
        }
    }

    fun selectServer(server: VpnServer) {
        _selectedServer.value = server
    }

    fun selectServerByCountry(countryCode: String): VpnServer? {
        val server = _servers.value.filter {
            !it.isWarp && it.countryShort.equals(countryCode, ignoreCase = true)
        }.minByOrNull { it.ping }
        if (server != null) {
            _selectedServer.value = server
        }
        return server
    }

    fun selectCountry(country: String?) {
        _selectedCountry.value = country
    }

    fun updateSearchQuery(query: String) {
        _searchQuery.value = query
    }

    fun setProtocolFilter(filter: ProtocolFilter) {
        _protocolFilter.value = filter
    }

    fun setSortMode(mode: SortMode) {
        _sortMode.value = mode
    }

    fun toggleConnection(context: Context, onRequirePermission: () -> Unit) {
        if (connectionState.value.status == ConnectionStatus.CONNECTED) {
            disconnect(context)
        } else {
            val vpnIntent = android.net.VpnService.prepare(context)
            if (vpnIntent != null) {
                onRequirePermission()
                return
            }
            val target = _selectedServer.value ?: repository.findBestServer(_servers.value)
            if (target != null) {
                connect(context, target, onRequirePermission)
            }
        }
    }

    fun connect(context: Context, server: VpnServer, onRequirePermission: () -> Unit) {
        val vpnIntent = android.net.VpnService.prepare(context)
        if (vpnIntent != null) {
            _selectedServer.value = server
            onRequirePermission()
            return
        }

        isConnectingByUser = true
        failoverCount = 0
        failedIps.clear()
        _selectedServer.value = server
        UnifiedTunnelManager.startVpn(context, server)
    }

    fun disconnect(context: Context) {
        isConnectingByUser = false
        failoverCount = 0
        failedIps.clear()
        UnifiedTunnelManager.stopVpn(context)
    }
}
