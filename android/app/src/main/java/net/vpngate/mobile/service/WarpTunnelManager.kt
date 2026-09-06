package net.vpngate.mobile.service

import android.content.Context
import android.content.SharedPreferences
import android.util.Log
import com.wireguard.android.backend.GoBackend
import com.wireguard.android.backend.Tunnel
import com.wireguard.config.Config
import com.wireguard.crypto.KeyPair
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import net.vpngate.mobile.data.model.ConnectionStatus
import net.vpngate.mobile.data.model.VpnConnectionState
import net.vpngate.mobile.data.model.VpnServer
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import java.io.ByteArrayInputStream
import java.util.concurrent.TimeUnit

object WarpTunnelManager : Tunnel {

    private const val TAG = "WireGuardTunnelManager"
    private const val TUNNEL_NAME = "OpenRelayWireGuard"
    private const val PREFS_NAME = "openrelay_wireguard_prefs"

    private const val PREF_PRIVATE_KEY = "priv_key"
    private const val PREF_PUBLIC_KEY = "pub_key"
    private const val PREF_V4_ADDR = "v4_addr"
    private const val PREF_V6_ADDR = "v6_addr"
    private const val PREF_PEER_PUB = "peer_pub"
    private const val PREF_ENDPOINT = "endpoint"

    private val scope = CoroutineScope(Dispatchers.Main + Job())
    private var statsJob: Job? = null
    private var timerJob: Job? = null

    private var backend: GoBackend? = null

    private val _connectionState = MutableStateFlow(VpnConnectionState())
    val connectionState = _connectionState.asStateFlow()

    private var currentServer: VpnServer? = null

    private val httpClient = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(15, TimeUnit.SECONDS)
        .build()

    override fun getName(): String = TUNNEL_NAME

    override fun onStateChange(newState: Tunnel.State) {
        Log.d(TAG, "WireGuard Tunnel state changed: $newState")
        when (newState) {
            Tunnel.State.UP -> {
                _connectionState.value = VpnConnectionState(
                    status = ConnectionStatus.CONNECTED,
                    connectedServer = currentServer ?: VpnServer.createWarpServer()
                )
                startStatsMonitoring()
                startTimer()
            }
            Tunnel.State.DOWN -> {
                stopMonitoring()
                _connectionState.value = VpnConnectionState(
                    status = ConnectionStatus.DISCONNECTED,
                    connectedServer = null
                )
            }
            Tunnel.State.TOGGLE -> {}
        }
    }

    private fun getBackend(context: Context): GoBackend {
        return backend ?: synchronized(this) {
            backend ?: GoBackend(context.applicationContext).also { backend = it }
        }
    }

    suspend fun startWarp(context: Context, server: VpnServer) = withContext(Dispatchers.IO) {
        currentServer = server
        _connectionState.value = VpnConnectionState(
            status = ConnectionStatus.CONNECTING,
            connectedServer = server
        )

        try {
            val b = getBackend(context)
            val prefs = context.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

            val warpConfig = ensureWarpRegistration(prefs)
            Log.d(TAG, "Starting WireGuard GoBackend with Anycast config...")

            b.setState(this@WarpTunnelManager, Tunnel.State.UP, warpConfig)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start WireGuard Anycast", e)
            _connectionState.value = VpnConnectionState(
                status = ConnectionStatus.ERROR,
                connectedServer = server,
                errorMessage = e.message ?: "Failed to connect to WireGuard Anycast"
            )
        }
    }

    suspend fun stopWarp(context: Context) = withContext(Dispatchers.IO) {
        try {
            val b = getBackend(context)
            b.setState(this@WarpTunnelManager, Tunnel.State.DOWN, null)
        } catch (e: Exception) {
            Log.w(TAG, "Error stopping WireGuard tunnel: ${e.message}")
        } finally {
            stopMonitoring()
            _connectionState.value = VpnConnectionState(
                status = ConnectionStatus.DISCONNECTED,
                connectedServer = null
            )
        }
    }

    private fun ensureWarpRegistration(prefs: SharedPreferences): Config {
        var privKey = prefs.getString(PREF_PRIVATE_KEY, null)
        var v4Addr = prefs.getString(PREF_V4_ADDR, null)
        var v6Addr = prefs.getString(PREF_V6_ADDR, null)
        var peerPub = prefs.getString(PREF_PEER_PUB, null)
        var endpoint = prefs.getString(PREF_ENDPOINT, null)

        if (privKey == null || v4Addr == null || peerPub == null || endpoint == null) {
            Log.d(TAG, "No cached Anycast registration found. Generating new WireGuard keypair and registering...")
            val keyPair = KeyPair()
            privKey = keyPair.privateKey.toBase64()
            val pubKey = keyPair.publicKey.toBase64()

            val regPayload = JSONObject().apply {
                put("key", pubKey)
                put("install_id", "")
                put("fcm_token", "")
                put("tos", "2020-09-01T00:00:00.000Z")
                put("model", "Android")
                put("serial_number", "")
                put("locale", "en_US")
            }

            val request = Request.Builder()
                .url("https://api.cloudflareclient.com/v0a2158/reg")
                .header("Content-Type", "application/json")
                .header("User-Agent", "okhttp/3.12.1")
                .post(regPayload.toString().toRequestBody("application/json".toMediaType()))
                .build()

            val responseBody = httpClient.newCall(request).execute().use { resp ->
                if (!resp.isSuccessful) {
                    throw IllegalStateException("Anycast relay registration failed: HTTP ${resp.code}")
                }
                resp.body?.string() ?: throw IllegalStateException("Empty registration response")
            }

            val json = JSONObject(responseBody)
            val configObj = json.getJSONObject("config")
            val peers = configObj.getJSONArray("peers")
            val firstPeer = peers.getJSONObject(0)
            peerPub = firstPeer.getString("public_key")

            val endpointObj = firstPeer.getJSONObject("endpoint")
            endpoint = if (endpointObj.has("host")) {
                endpointObj.getString("host")
            } else {
                "162.159.192.1:2408"
            }

            val ifaceObj = configObj.getJSONObject("interface")
            val addressesObj = ifaceObj.getJSONObject("addresses")
            v4Addr = addressesObj.getString("v4")
            v6Addr = addressesObj.optString("v6", "")

            prefs.edit()
                .putString(PREF_PRIVATE_KEY, privKey)
                .putString(PREF_PUBLIC_KEY, pubKey)
                .putString(PREF_V4_ADDR, v4Addr)
                .putString(PREF_V6_ADDR, v6Addr)
                .putString(PREF_PEER_PUB, peerPub)
                .putString(PREF_ENDPOINT, endpoint)
                .apply()

            Log.d(TAG, "Anycast relay registered successfully. Assigned IP: $v4Addr")
        }

        val addressSection = if (!v6Addr.isNullOrBlank()) {
            "$v4Addr/32, $v6Addr/128"
        } else {
            "$v4Addr/32"
        }

        val configText = buildString {
            appendLine("[Interface]")
            appendLine("PrivateKey = $privKey")
            appendLine("Address = $addressSection")
            appendLine("DNS = 1.1.1.1, 1.0.0.1")
            appendLine()
            appendLine("[Peer]")
            appendLine("PublicKey = $peerPub")
            appendLine("Endpoint = $endpoint")
            appendLine("AllowedIPs = 0.0.0.0/0, ::/0")
        }

        return Config.parse(ByteArrayInputStream(configText.toByteArray(Charsets.UTF_8)))
    }

    private fun startStatsMonitoring() {
        statsJob?.cancel()
        statsJob = scope.launch {
            while (isActive) {
                delay(1500)
                try {
                    val b = backend ?: continue
                    val stats = b.getStatistics(this@WarpTunnelManager)
                    val totalRx = stats.totalRx()
                    val totalTx = stats.totalTx()
                    _connectionState.value = _connectionState.value.copy(
                        bytesIn = totalRx,
                        bytesOut = totalTx
                    )
                } catch (_: Exception) {}
            }
        }
    }

    private fun startTimer() {
        timerJob?.cancel()
        timerJob = scope.launch {
            var seconds = 0L
            while (isActive) {
                delay(1000)
                seconds++
                _connectionState.value = _connectionState.value.copy(durationSeconds = seconds)
            }
        }
    }

    private fun stopMonitoring() {
        statsJob?.cancel()
        statsJob = null
        timerJob?.cancel()
        timerJob = null
    }
}
