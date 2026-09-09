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

                val merged = (listOf(warp) + vpnBookServers + parsed).distinctBy {
                    "${it.source}_${it.ip}_${it.remotePort}_${it.protocol}"
                }
                cachedServers = merged
                isLiveLoaded = true
                Log.d("OpenRelay", "Total unique servers available: ${merged.size} (VPNGate=${parsed.size}, VPNBook=${vpnBookServers.size}, WireGuard=1)")
                return@withContext merged
            }
        } catch (e: Exception) {
            Log.e("VPNGate", "Error fetching dynamic live servers: ${e.message}")
            if (cachedServers.isNotEmpty()) {
                return@withContext cachedServers
            }
            val bundled = getBundledServers(context)
            val fallback = (listOf(warp) + vpnBookServers + bundled).distinctBy {
                "${it.source}_${it.ip}_${it.remotePort}_${it.protocol}"
            }
            cachedServers = fallback
            return@withContext fallback
        }

        return@withContext cachedServers
    }

    suspend fun pingServers(
        servers: List<VpnServer>,
        onProgress: (suspend (List<VpnServer>) -> Unit)? = null
    ): List<VpnServer> = withContext(Dispatchers.IO) {
        val currentList = servers.toMutableList()

        // Prioritize VPNBook + WARP + Backbone + top 30 VPNGate servers
        val priorityTargets = servers.filter { it.isWarp || it.isVpnBook || it.isBackbone } +
                servers.filter { !it.isWarp && !it.isVpnBook && !it.isBackbone }.take(30)

        // Gentle concurrency of 6 parallel sockets, 2000ms timeout
        val chunks = priorityTargets.chunked(6)
        for (chunk in chunks) {
            val batchResults = chunk.map { server ->
                async {
                    if (server.isWarp) {
                        server.copy(isReachable = true, verifiedLatency = 15L)
                    } else {
                        val port = if (server.remotePort in 1..65535) server.remotePort else 443
                        val ms = PingUtil.measureLatency(
                            host = server.ip,
                            port = port,
                            timeoutMs = 2000
                        )
                        if (ms > 0) {
                            server.copy(isReachable = true, verifiedLatency = ms)
                        } else {
                            server.copy(isReachable = false)
                        }
                    }
                }
            }.awaitAll()

            val batchMap = batchResults.associateBy { it.ip + it.source + it.remotePort }
            for (i in currentList.indices) {
                val key = currentList[i].ip + currentList[i].source + currentList[i].remotePort
                batchMap[key]?.let { currentList[i] = it }
            }
            onProgress?.invoke(currentList.toList())
        }
        currentList.toList()
    }

    suspend fun getCurrentPublicIp(): String? = apiService.fetchCurrentPublicIp()

    fun findBestServer(servers: List<VpnServer>): VpnServer? {
        val warp = servers.firstOrNull { it.isWarp }
        val backbone = servers.filter { it.isBackbone && it.isOnline }
            .minByOrNull { it.ping }
        return warp ?: backbone ?: servers.filter { it.isOnline }.minByOrNull { it.ping }
    }
}
