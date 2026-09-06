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

    private var activeProtocol: String? = null // "wireguard" or "openvpn"

    init {
        scope.launch {
            WarpTunnelManager.connectionState.collect { state ->
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
    }

    fun startVpn(context: Context, server: VpnServer) {
        Log.d(TAG, "startVpn called for ${server.countryLong} (proto=${server.protocol}, source=${server.source})")

        // Disconnect whichever tunnel might be running first
        if (activeProtocol == "wireguard" && !server.isWarp) {
            scope.launch { WarpTunnelManager.stopWarp(context) }
        } else if (activeProtocol == "openvpn" && server.isWarp) {
            OpenVpnTunnelManager.stopVpn()
        }

        if (server.isWarp) {
            activeProtocol = "wireguard"
            _connectionState.value = VpnConnectionState(
                status = ConnectionStatus.CONNECTING,
                connectedServer = server
            )
            scope.launch {
                WarpTunnelManager.startWarp(context, server)
            }
        } else {
            activeProtocol = "openvpn"
            _connectionState.value = VpnConnectionState(
                status = ConnectionStatus.CONNECTING,
                connectedServer = server
            )
            OpenVpnTunnelManager.startVpn(context, server)
        }
    }

    fun setDetectedPublicIp(ip: String) {
        // Guard: only emit a new state if the IP actually changed
        if (_connectionState.value.detectedPublicIp == ip) return
        _connectionState.value = _connectionState.value.copy(detectedPublicIp = ip)
    }

    fun stopVpn(context: Context) {
        Log.d(TAG, "stopVpn called")
        if (activeProtocol == "wireguard") {
            scope.launch {
                WarpTunnelManager.stopWarp(context)
            }
        } else {
            OpenVpnTunnelManager.stopVpn()
        }
        activeProtocol = null
        _connectionState.value = VpnConnectionState(
            status = ConnectionStatus.DISCONNECTED,
            connectedServer = null
        )
    }
}
