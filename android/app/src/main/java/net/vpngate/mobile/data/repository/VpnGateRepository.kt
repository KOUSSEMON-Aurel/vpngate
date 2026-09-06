package net.vpngate.mobile.data.repository

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.withContext
import net.vpngate.mobile.data.api.VpnGateApiService
import net.vpngate.mobile.data.model.VpnServer
import net.vpngate.mobile.util.CsvParser
import net.vpngate.mobile.util.PingUtil

class VpnGateRepository(
    private val apiService: VpnGateApiService = VpnGateApiService()
) {
    private var cachedServers: List<VpnServer> = emptyList()

    suspend fun getServers(forceRefresh: Boolean = false): List<VpnServer> = withContext(Dispatchers.IO) {
        if (!forceRefresh && cachedServers.isNotEmpty()) {
            return@withContext cachedServers
        }

        try {
            val rawCsv = apiService.fetchServerListRaw()
            val parsed = CsvParser.parseVpnList(rawCsv)
            if (parsed.isNotEmpty()) {
                cachedServers = parsed
                return@withContext parsed
            }
        } catch (e: Exception) {
            if (cachedServers.isNotEmpty()) {
                return@withContext cachedServers
            }
            throw e
        }

        return@withContext cachedServers
    }

    suspend fun pingServers(servers: List<VpnServer>, limit: Int = 10): List<VpnServer> = withContext(Dispatchers.IO) {
        val targets = servers.take(limit)
        val pinged = targets.map { server ->
            async {
                val latency = PingUtil.measureLatency(server.ip)
                if (latency > 0) {
                    server.copy(ping = latency)
                } else {
                    server
                }
            }
        }.awaitAll()

        // Merge back into servers list
        val pingMap = pinged.associateBy { it.ip }
        servers.map { pingMap[it.ip] ?: it }
    }

    fun findBestServer(servers: List<VpnServer>): VpnServer? {
        return servers.filter { it.openVpnConfigDataBase64.isNotBlank() }
            .sortedWith(
                compareBy<VpnServer> { it.ping.takeIf { p -> p > 0 } ?: 999 }
                    .thenByDescending { it.speed }
                    .thenByDescending { it.score }
            )
            .firstOrNull()
    }
}
