package net.vpngate.mobile.data.repository

import android.content.Context
import android.util.Log
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.withContext
import net.vpngate.mobile.data.api.VpnGateApiService
import net.vpngate.mobile.data.model.VpnServer
import net.vpngate.mobile.util.CsvParser
import net.vpngate.mobile.util.PingUtil
import java.io.File

class VpnGateRepository(
    private val apiService: VpnGateApiService = VpnGateApiService()
) {
    private var cachedServers: List<VpnServer> = emptyList()
    private var isLiveLoaded = false
    private val cacheFileName = "vpngate_cache.csv"

    fun getInitialServers(context: Context): List<VpnServer> {
        val warp = VpnServer.createWarpServer()

        // 1. Try disk cache from previous app run
        try {
            val cacheFile = File(context.filesDir, cacheFileName)
            if (cacheFile.exists() && cacheFile.length() > 500) {
                val cachedCsv = cacheFile.readText()
                val parsed = CsvParser.parseVpnList(cachedCsv)
                if (parsed.isNotEmpty()) {
                    Log.d("VPNGate", "Loaded ${parsed.size} cached live servers from disk")
                    val result = listOf(warp) + parsed
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
            listOf(warp) + bundled
        } else {
            listOf(warp)
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

                val merged = listOf(warp) + parsed
                cachedServers = merged
                isLiveLoaded = true
                Log.d("VPNGate", "Total servers available: ${merged.size} (including WARP)")
                return@withContext merged
            }
        } catch (e: Exception) {
            Log.e("VPNGate", "Error fetching dynamic live servers: ${e.message}")
            if (cachedServers.isNotEmpty()) {
                return@withContext cachedServers
            }
            val bundled = getBundledServers(context)
            val fallback = listOf(warp) + bundled
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
                    val latency = PingUtil.measureLatency(server.ip, server.remotePort, timeoutMs = 1200)
                    if (latency > 0) {
                        server.copy(ping = latency)
                    } else {
                        server.copy(ping = 9999L)
                    }
                }
            }
        }.awaitAll()

        val pingMap = pinged.associateBy { it.ip }
        servers.map { pingMap[it.ip] ?: it }
    }

    fun findBestServer(servers: List<VpnServer>): VpnServer? {
        // Filter out dead/unreachable servers
        val reachable = servers.filter { it.ping < 9000L }
        val openVpnServers = reachable.filter { it.openVpnConfigDataBase64.isNotBlank() }

        val bestOpenVpn = openVpnServers.sortedWith(
            compareByDescending<VpnServer> { it.isBackbone }
                .thenByDescending { it.isPort443 }
                .thenByDescending { it.speed }
                .thenByDescending { it.score }
                .thenBy { it.ping.takeIf { p -> p > 0 } ?: 999 }
        ).firstOrNull()

        if (bestOpenVpn != null) {
            return bestOpenVpn
        }

        return reachable.firstOrNull() ?: servers.firstOrNull()
    }
}
