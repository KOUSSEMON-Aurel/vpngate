package net.vpngate.mobile.util

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.net.InetSocketAddress
import java.net.Socket

object PingUtil {

    suspend fun measureLatency(host: String, port: Int = 443, timeoutMs: Int = 1200): Long =
        withContext(Dispatchers.IO) {
            val start = System.currentTimeMillis()
            try {
                Socket().use { socket ->
                    socket.connect(InetSocketAddress(host, port), timeoutMs)
                }
                System.currentTimeMillis() - start
            } catch (_: Exception) {
                -1L
            }
        }
}
