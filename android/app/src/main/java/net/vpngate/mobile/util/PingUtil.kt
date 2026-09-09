package net.vpngate.mobile.util

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import java.net.InetSocketAddress
import java.net.Socket

object PingUtil {

    /**
     * Measures round-trip TCP connect latency to the target host and port.
     * Pure socket connection with zero external processes and zero background thread bloat.
     */
    suspend fun measureLatency(
        host: String,
        port: Int = 443,
        timeoutMs: Int = 2000
    ): Long = withContext(Dispatchers.IO) {
        val targetPort = if (port in 1..65535) port else 443
        val start = System.currentTimeMillis()
        try {
            Socket().use { socket ->
                socket.tcpNoDelay = true
                socket.soTimeout = timeoutMs
                socket.connect(InetSocketAddress(host, targetPort), timeoutMs)
            }
            (System.currentTimeMillis() - start).coerceAtLeast(1L)
        } catch (_: Exception) {
            -1L
        }
    }
}
