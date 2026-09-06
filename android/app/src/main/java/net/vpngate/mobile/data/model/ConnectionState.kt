package net.vpngate.mobile.data.model

enum class ConnectionStatus {
    DISCONNECTED,
    CONNECTING,
    CONNECTED,
    DISCONNECTING,
    ERROR
}

data class VpnConnectionState(
    val status: ConnectionStatus = ConnectionStatus.DISCONNECTED,
    val connectedServer: VpnServer? = null,
    val durationSeconds: Long = 0,
    val bytesIn: Long = 0,
    val bytesOut: Long = 0,
    val currentPingMs: Long = 0,
    val errorMessage: String? = null
)
