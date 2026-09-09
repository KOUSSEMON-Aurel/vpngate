package net.vpngate.mobile.service

import android.content.Context
import android.util.Log
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import net.vpngate.mobile.data.model.ConnectionStatus
import net.vpngate.mobile.data.model.VpnConnectionState
import net.vpngate.mobile.data.model.VpnServer

object UnifiedTunnelManager {

    private const val TAG = "UnifiedTunnelManager"
    private val scope = CoroutineScope(Dispatchers.Main + Job())

    private val _connectionState = MutableStateFlow(VpnConnectionState())
    val connectionState = _connectionState.asStateFlow()

    private var appContext: Context? = null
    private var activeProtocol: String? = null // "wireguard" or "openvpn"

    fun init(context: Context) {
        appContext = context.applicationContext
    }

    init {
        scope.launch {
            WireGuardTunnelManager.connectionState.collect { state ->
                if (activeProtocol == "wireguard") {
                    val currentIp = if (state.status == ConnectionStatus.CONNECTED) {
                        _connectionState.value.detectedPublicIp
                    } else null
                    _connectionState.value = state.copy(detectedPublicIp = currentIp)
                }
            }
        }

        scope.launch {
            OpenVpnTunnelManager.connectionState.collect { state ->
                if (activeProtocol == "openvpn") {
                    val currentIp = if (state.status == ConnectionStatus.CONNECTED) {
                        _connectionState.value.detectedPublicIp
                    } else null
                    _connectionState.value = state.copy(detectedPublicIp = currentIp)
                }
            }
        }

        scope.launch {
            _connectionState.collect { state ->
                val ctx = appContext ?: return@collect
                when (state.status) {
                    ConnectionStatus.CONNECTING -> {
                        VpnNotificationManager.showConnecting(ctx, state.connectedServer)
                    }
                    ConnectionStatus.CONNECTED -> {
                        VpnNotificationManager.showConnected(ctx, state.connectedServer, state.detectedPublicIp)
                    }
                    ConnectionStatus.DISCONNECTED, ConnectionStatus.DISCONNECTING, ConnectionStatus.ERROR -> {
                        VpnNotificationManager.dismiss(ctx)
                    }
                }
            }
        }
    }

    fun startVpn(context: Context, server: VpnServer) {
        appContext = context.applicationContext
        Log.d(TAG, "startVpn called for ${server.countryLong} (proto=${server.protocol}, source=${server.source})")

        scope.launch {
            // Cleanly stop whichever tunnel might be running first to avoid TUN collision
            val prevProto = activeProtocol
            if (prevProto == "wireguard") {
                WireGuardTunnelManager.stopTunnel(context)
                kotlinx.coroutines.delay(350)
            } else if (prevProto == "openvpn") {
                OpenVpnTunnelManager.stopVpn()
                kotlinx.coroutines.delay(350)
            }

            if (server.isWarp) {
                activeProtocol = "wireguard"
                _connectionState.value = VpnConnectionState(
                    status = ConnectionStatus.CONNECTING,
                    connectedServer = server
                )
                WireGuardTunnelManager.startTunnel(context, server)
            } else {
                activeProtocol = "openvpn"
                _connectionState.value = VpnConnectionState(
                    status = ConnectionStatus.CONNECTING,
                    connectedServer = server
                )
                OpenVpnTunnelManager.startVpn(context, server)
            }
        }
    }

    fun setDetectedPublicIp(ip: String) {
        // Guard: only emit a new state if the IP actually changed
        if (_connectionState.value.detectedPublicIp == ip) return
        _connectionState.value = _connectionState.value.copy(detectedPublicIp = ip)
        appContext?.let { ctx ->
            if (_connectionState.value.status == ConnectionStatus.CONNECTED) {
                VpnNotificationManager.showConnected(ctx, _connectionState.value.connectedServer, ip)
            }
        }
    }

    fun stopVpn(context: Context) {
        appContext = context.applicationContext
        Log.d(TAG, "stopVpn called")
        if (activeProtocol == "wireguard") {
            scope.launch {
                WireGuardTunnelManager.stopTunnel(context)
            }
        } else {
            OpenVpnTunnelManager.stopVpn()
        }
        activeProtocol = null
        _connectionState.value = VpnConnectionState(
            status = ConnectionStatus.DISCONNECTED,
            connectedServer = null
        )
        VpnNotificationManager.dismiss(context)
    }
}
