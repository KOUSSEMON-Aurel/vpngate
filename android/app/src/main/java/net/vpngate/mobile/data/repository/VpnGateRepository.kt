package net.vpngate.mobile.data.repository

import android.content.Context
import android.util.Log
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.withContext
import net.vpngate.mobile.data.api.VpnBookApiService
import net.vpngate.mobile.data.api.VpnGateApiService
import net.vpngate.mobile.data.model.VpnServer
import net.vpngate.mobile.util.CsvParser
import net.vpngate.mobile.util.PingUtil
import java.io.File

class VpnGateRepository(
    private val apiService: VpnGateApiService = VpnGateApiService(),
    private val vpnBookService: VpnBookApiService = VpnBookApiService()
) {
    private var cachedServers: List<VpnServer> = emptyList()
    private var isLiveLoaded = false
    private val cacheFileName = "vpngate_cache.csv"

    fun getInitialServers(context: Context): List<VpnServer> {
        val warp = VpnServer.createWarpServer()
        val vpnBookServers = vpnBookService.loadBundledServers(context)

        // 1. Try disk cache from previous app run
        try {
            val cacheFile = File(context.filesDir, cacheFileName)
            if (cacheFile.exists() && cacheFile.length() > 500) {
                val cachedCsv = cacheFile.readText()
                val parsed = CsvParser.parseVpnList(cachedCsv)
                if (parsed.isNotEmpty()) {
                    Log.d("VPNGate", "Loaded ${parsed.size} cached live servers from disk")
                    val result = listOf(warp) + vpnBookServers + parsed
                    cachedServers = result
                    return result
                }
            }
        } catch (e: Exception) {
            Log.w("VPNGate", "Could not read disk cache: ${e.message}")
        }

        // 2. Fall back to bundled initial asset
        val bundled = getBundledServers(context)
        val initialList = if (bundled.isNotEmpty()) {
            listOf(warp) + vpnBookServers + bundled
        } else {
            listOf(warp) + vpnBookServers
        }
        cachedServers = initialList
        return initialList
    }

    fun getBundledServers(context: Context): List<VpnServer> {
        return try {
            val stream = context.assets.open("default_servers.csv")
            val content = stream.bufferedReader().use { it.readText() }
            val parsed = CsvParser.parseVpnList(content)
            Log.d("VPNGate", "Loaded ${parsed.size} bundled default servers from assets")
            parsed
        } catch (e: Exception) {
            Log.w("VPNGate", "Failed to load bundled servers: ${e.message}")
            emptyList()
        }
    }

    suspend fun getServers(context: Context, forceRefresh: Boolean = false): List<VpnServer> = withContext(Dispatchers.IO) {
        // If already loaded live servers and not forcing refresh, return in-memory cache
        if (!forceRefresh && isLiveLoaded && cachedServers.size > 1) {
            return@withContext cachedServers
        }

        val warp = VpnServer.createWarpServer()
        val vpnBookServers = vpnBookService.getServers(context)

        try {
            Log.d("VPNGate", "Fetching live server list dynamically from network...")
            val rawCsv = apiService.fetchServerListRaw()
            val parsed = CsvParser.parseVpnList(rawCsv)
            if (parsed.isNotEmpty()) {
                // Save fresh CSV to phone storage
                try {
                    val cacheFile = File(context.filesDir, cacheFileName)
                    cacheFile.writeText(rawCsv)
                    Log.d("VPNGate", "Saved ${parsed.size} live servers to disk cache: ${cacheFile.absolutePath}")
                } catch (ce: Exception) {
                    Log.w("VPNGate", "Failed writing cache file: ${ce.message}")
                }

                val merged = listOf(warp) + vpnBookServers + parsed
                cachedServers = merged
                isLiveLoaded = true
                Log.d("OpenRelay", "Total servers available: ${merged.size} (VPNGate=${parsed.size}, VPNBook=${vpnBookServers.size}, WireGuard=1)")
                return@withContext merged
            }
        } catch (e: Exception) {
            Log.e("VPNGate", "Error fetching dynamic live servers: ${e.message}")
            if (cachedServers.isNotEmpty()) {
                return@withContext cachedServers
            }
            val bundled = getBundledServers(context)
            val fallback = listOf(warp) + vpnBookServers + bundled
            cachedServers = fallback
            return@withContext fallback
        }

        return@withContext cachedServers
    }

    suspend fun pingServers(servers: List<VpnServer>, limit: Int = 40): List<VpnServer> = withContext(Dispatchers.IO) {
        val targets = servers.take(limit)
        val pinged = targets.map { server ->
            async {
                if (server.isWarp) {
                    server.copy(ping = 15)
                } else {
                    val ms = PingUtil.measureLatency(server.ip, server.remotePort, 1500)
                    server.copy(ping = if (ms > 0) ms else server.ping)
                }
            }
        }.awaitAll()

        val pingMap = pinged.associateBy { it.hostName }
        servers.map { original ->
            pingMap[original.hostName] ?: original
        }
    }

    suspend fun getCurrentPublicIp(): String? = apiService.fetchCurrentPublicIp()

    fun findBestServer(servers: List<VpnServer>): VpnServer? {
        val warp = servers.firstOrNull { it.isWarp }
        val backbone = servers.filter { it.isBackbone && it.isOnline }
            .minByOrNull { it.ping }
        return backbone ?: warp ?: servers.filter { it.isOnline }.minByOrNull { it.ping }
    }
}
