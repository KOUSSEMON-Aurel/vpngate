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
            // Allow up to 12s for initial server handshake
            delay(12_000)
            if (_connectionState.value.status == ConnectionStatus.CONNECTING) {
                Log.w(TAG, "Initial connection timeout (12s) reached. Stopping tunnel.")
                stopVpnInternal(isError = true)
                _connectionState.value = VpnConnectionState(
                    status = ConnectionStatus.ERROR,
                    connectedServer = server,
                    errorMessage = "Server ${server.ip} did not respond."
                )
            }
        }

        try {
            Log.d(TAG, "Starting OpenVPN for ${server.countryLong} (${server.ip})...")
            val sanitizedConfig = buildString {
                appendLine("redirect-gateway def1")
                appendLine("dhcp-option DNS 1.1.1.1")
                appendLine("dhcp-option DNS 8.8.8.8")
                appendLine("tun-mtu 1500")
                appendLine("mssfix 1450")
                appendLine("nobind")
                appendLine("auth-user-pass")
                appendLine("connect-retry 1")
                appendLine("connect-retry-max 1")
                appendLine("resolv-retry 5")
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

    private fun stopVpnInternal(isError: Boolean) {
        timeoutJob?.cancel()
        try {
            OpenVPNThread.stop()
        } catch (e: Exception) {
            Log.w(TAG, "Error stopping OpenVPNThread: ${e.message}")
        }
        timerJob?.cancel()
        if (!isError) {
            _connectionState.value = VpnConnectionState(
                status = ConnectionStatus.DISCONNECTED,
                connectedServer = null
            )
        }
    }

    fun stopVpn() {
        Log.d(TAG, "Stopping OpenVPN...")
        stopVpnInternal(isError = false)
    }

    override fun updateState(
        state: String?,
        logmessage: String?,
        localizedResId: Int,
        level: LibStatus?,
        intent: Intent?
    ) {
        Log.d(TAG, "OpenVPN state update: level=$level, state=$state, msg=$logmessage")

        // Handle successful connection
        if (level == LibStatus.LEVEL_CONNECTED || state == "CONNECTED") {
            Log.i(TAG, "OpenVPN connection established successfully!")
            timeoutJob?.cancel()
            _connectionState.value = _connectionState.value.copy(
                status = ConnectionStatus.CONNECTED,
                connectedServer = currentTargetServer
            )
            startTimer()
            return
        }

        when (level) {
            LibStatus.LEVEL_CONNECTING_SERVER_REPLIED -> {
                // Server responded! Extend timeout to allow full TLS handshake and route/push configuration
                timeoutJob?.cancel()
                timeoutJob = scope.launch {
                    delay(25_000)
                    if (_connectionState.value.status == ConnectionStatus.CONNECTING) {
                        Log.w(TAG, "Tunnel configuration timeout (25s) reached.")
                        stopVpnInternal(isError = true)
                        _connectionState.value = VpnConnectionState(
                            status = ConnectionStatus.ERROR,
                            connectedServer = currentTargetServer,
                            errorMessage = "Server ${currentTargetServer?.ip} timed out during configuration."
                        )
                    }
                }
                _connectionState.value = _connectionState.value.copy(
                    status = ConnectionStatus.CONNECTING,
                    connectedServer = currentTargetServer
                )
            }
            LibStatus.LEVEL_START,
            LibStatus.LEVEL_CONNECTING_NO_SERVER_REPLY_YET -> {
                if (state == "RECONNECTING") {
                    Log.w(TAG, "OpenVPN connection attempt failed and triggered RECONNECTING.")
                    timeoutJob?.cancel()
                    stopVpnInternal(isError = true)
                    _connectionState.value = VpnConnectionState(
                        status = ConnectionStatus.ERROR,
                        connectedServer = currentTargetServer,
                        errorMessage = "Relay ${currentTargetServer?.ip} refused connection."
                    )
                } else {
                    _connectionState.value = _connectionState.value.copy(
                        status = ConnectionStatus.CONNECTING,
                        connectedServer = currentTargetServer
                    )
                }
            }
            LibStatus.LEVEL_NOTCONNECTED,
            LibStatus.LEVEL_NONETWORK -> {
                // Do not overwrite an ERROR status with DISCONNECTED when the process exits after an error
                if (_connectionState.value.status != ConnectionStatus.ERROR) {
                    timerJob?.cancel()
                    _connectionState.value = VpnConnectionState(
                        status = ConnectionStatus.DISCONNECTED,
                        connectedServer = null
                    )
                }
            }
            LibStatus.LEVEL_AUTH_FAILED -> {
                timerJob?.cancel()
                timeoutJob?.cancel()
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
