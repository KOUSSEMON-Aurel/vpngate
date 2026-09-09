package net.vpngate.mobile.data.model

import android.util.Base64

data class VpnServer(
    val hostName: String,
    val ip: String,
    val score: Long,
    val ping: Long,
    val speed: Long,
    val countryLong: String,
    val countryShort: String,
    val numVpnSessions: Long,
    val uptime: Long,
    val totalUsers: Long,
    val totalTraffic: Long,
    val logType: String,
    val operator: String,
    val message: String,
    val openVpnConfigDataBase64: String,
    val source: String = "vpngate",
    val protocol: String = "openvpn",
    val authUsername: String? = null,
    val authPassword: String? = null,
    val verifiedLatency: Long? = null,
    val isReachable: Boolean? = null
) {
    val isWireguard: Boolean
        get() = protocol == "wireguard" || source == "wireguard" || source == "warp"

    val isWarp: Boolean
        get() = isWireguard

    val isVpnBook: Boolean
        get() = source == "vpnbook"

    val countryBadge: String
        get() = if (isWarp) "WG" else countryShort.uppercase().take(3).ifBlank { "VPN" }

    val decodedConfig: String by lazy {
        try {
            if (openVpnConfigDataBase64.isNotBlank()) {
                String(Base64.decode(openVpnConfigDataBase64, Base64.DEFAULT))
            } else ""
        } catch (e: Exception) {
            ""
        }
    }

    val speedMbps: Double
        get() = (speed.toDouble() / (1024 * 1024)).coerceAtLeast(0.0)

    val isPort443: Boolean
        get() = decodedConfig.contains(" 443\n") || decodedConfig.contains(" 443\r\n") || decodedConfig.contains(" 443 ")

    val isBackbone: Boolean
        get() = ip.startsWith("219.100.37.")

    val remotePort: Int by lazy {
        val match = Regex("""remote\s+[\w\.-]+\s+(\d+)""").find(decodedConfig)
        match?.groupValues?.get(1)?.toIntOrNull() ?: 443
    }

    val isUdp: Boolean by lazy {
        decodedConfig.contains("proto udp")
    }

    val isOnline: Boolean
        get() = if (isWarp) true else isReachable != false

    val effectivePing: Long
        get() = if (isWarp) 15L else (verifiedLatency ?: (if (ping in 1..8999) ping else 999L))

    companion object {
        fun createWarpServer(): VpnServer {
            return VpnServer(
                hostName = "anycast-relay",
                ip = "162.159.192.1",
                score = 999999,
                ping = 15,
                speed = 1000 * 1024 * 1024L,
                countryLong = "Anycast Fast Relay (WireGuard)",
                countryShort = "WG",
                numVpnSessions = 10000,
                uptime = 9999999,
                totalUsers = 5000000,
                totalTraffic = 1000000000,
                logType = "none",
                operator = "Global Anycast Backbone",
                message = "WireGuard Protocol",
                openVpnConfigDataBase64 = "",
                source = "wireguard",
                protocol = "wireguard"
            )
        }

        fun createAnycastServer(): VpnServer = createWarpServer()
    }
}
