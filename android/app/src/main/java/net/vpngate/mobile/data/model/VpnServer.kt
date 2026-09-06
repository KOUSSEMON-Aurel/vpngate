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
    val openVpnConfigDataBase64: String
) {
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

    val flagEmoji: String
        get() = countryCodeToEmoji(countryShort)

    companion object {
        fun countryCodeToEmoji(countryCode: String): String {
            if (countryCode.length != 2) return "🌐"
            val upper = countryCode.uppercase()
            val firstChar = Character.codePointAt(upper, 0) - 0x41 + 0x1F1E6
            val secondChar = Character.codePointAt(upper, 1) - 0x41 + 0x1F1E6
            return String(Character.toChars(firstChar)) + String(Character.toChars(secondChar))
        }
    }
}
