package net.vpngate.mobile.service

import android.content.Context
import android.content.Intent
import android.util.Log
import de.blinkt.openvpn.OpenVpnApi
import de.blinkt.openvpn.core.ConnectionStatus as LibStatus
import de.blinkt.openvpn.core.OpenVPNThread
import de.blinkt.openvpn.core.VpnStatus
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import net.vpngate.mobile.data.model.ConnectionStatus
import net.vpngate.mobile.data.model.VpnConnectionState
import net.vpngate.mobile.data.model.VpnServer

object OpenVpnTunnelManager : VpnStatus.StateListener, VpnStatus.ByteCountListener {

    private const val TAG = "OpenVpnTunnelManager"
    private val scope = CoroutineScope(Dispatchers.Main + Job())
    private var timerJob: Job? = null
    private var timeoutJob: Job? = null

    private val _connectionState = MutableStateFlow(VpnConnectionState())
    val connectionState = _connectionState.asStateFlow()

    private var currentTargetServer: VpnServer? = null

    init {
        VpnStatus.addStateListener(this)
        VpnStatus.addByteCountListener(this)
        VpnStatus.addLogListener { item ->
            try {
                Log.d("OpenVPNLog", "[${item.logLevel}] ${item.getString(null)}")
            } catch (_: Exception) {}
        }
    }

    fun startVpn(context: Context, server: VpnServer) {
        currentTargetServer = server
        _connectionState.value = VpnConnectionState(
            status = ConnectionStatus.CONNECTING,
            connectedServer = server
        )

        timeoutJob?.cancel()
        timeoutJob = scope.launch {
            delay(15000)
            if (_connectionState.value.status == ConnectionStatus.CONNECTING) {
                Log.w(TAG, "Connection timeout (15s) reached. Stopping tunnel.")
                stopVpn()
                _connectionState.value = VpnConnectionState(
                    status = ConnectionStatus.ERROR,
                    connectedServer = server,
                    errorMessage = "Server unreachable. Please select another relay."
                )
            }
        }

        try {
            Log.d(TAG, "Starting OpenVPN for ${server.countryLong} (${server.ip})...")
            val sanitizedConfig = buildString {
                appendLine("redirect-gateway def1")
                appendLine("dhcp-option DNS 1.1.1.1")
                appendLine("dhcp-option DNS 8.8.8.8")
                appendLine("block-outside-dns")
                appendLine("tun-mtu 1400")
                appendLine("mssfix 1360")
                appendLine("nobind")
                appendLine("auth-user-pass")
                appendLine("connect-retry 2")
                appendLine("connect-retry-max 2")
                appendLine("resolv-retry 5")
                appendLine("server-poll-timeout 10")
                appendLine()
                append(server.decodedConfig)
            }
            OpenVpnApi.startVpn(
                context,
                sanitizedConfig,
                server.countryLong,
                "vpn",
                "vpn",
                emptyList()
            )
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start OpenVPN", e)
            timeoutJob?.cancel()
            _connectionState.value = VpnConnectionState(
                status = ConnectionStatus.ERROR,
                connectedServer = server,
                errorMessage = e.message ?: "Tunnel launch failed"
            )
        }
    }

    fun stopVpn() {
        Log.d(TAG, "Stopping OpenVPN...")
        timeoutJob?.cancel()
        try {
            OpenVPNThread.stop()
        } catch (e: Exception) {
            Log.w(TAG, "Error stopping OpenVPNThread: ${e.message}")
        }
        timerJob?.cancel()
        _connectionState.value = VpnConnectionState(
            status = ConnectionStatus.DISCONNECTED,
            connectedServer = null
        )
    }

    override fun updateState(
        state: String?,
        logmessage: String?,
        localizedResId: Int,
        level: LibStatus?,
        intent: Intent?
    ) {
        Log.d(TAG, "OpenVPN state update: level=$level, state=$state, msg=$logmessage")

        when (level) {
            LibStatus.LEVEL_CONNECTED -> {
                timeoutJob?.cancel()
                _connectionState.value = _connectionState.value.copy(
                    status = ConnectionStatus.CONNECTED,
                    connectedServer = currentTargetServer
                )
                startTimer()
            }
            LibStatus.LEVEL_START,
            LibStatus.LEVEL_CONNECTING_SERVER_REPLIED,
            LibStatus.LEVEL_CONNECTING_NO_SERVER_REPLY_YET -> {
                _connectionState.value = _connectionState.value.copy(
                    status = ConnectionStatus.CONNECTING,
                    connectedServer = currentTargetServer
                )
            }
            LibStatus.LEVEL_NOTCONNECTED,
            LibStatus.LEVEL_NONETWORK -> {
                timerJob?.cancel()
                _connectionState.value = VpnConnectionState(
                    status = ConnectionStatus.DISCONNECTED,
                    connectedServer = null
                )
            }
            LibStatus.LEVEL_AUTH_FAILED -> {
                timerJob?.cancel()
                _connectionState.value = VpnConnectionState(
                    status = ConnectionStatus.ERROR,
                    connectedServer = currentTargetServer,
                    errorMessage = "Authentication failed"
                )
            }
            else -> {}
        }
    }

    override fun setConnectedVPN(uuid: String?) {}

    override fun updateByteCount(inBytes: Long, outBytes: Long, diffIn: Long, diffOut: Long) {
        _connectionState.value = _connectionState.value.copy(
            bytesIn = inBytes,
            bytesOut = outBytes
        )
    }

    private fun startTimer() {
        timerJob?.cancel()
        timerJob = scope.launch {
            var elapsed = 0L
            while (isActive && _connectionState.value.status == ConnectionStatus.CONNECTED) {
                delay(1000)
                elapsed++
                _connectionState.value = _connectionState.value.copy(durationSeconds = elapsed)
            }
        }
    }
}
