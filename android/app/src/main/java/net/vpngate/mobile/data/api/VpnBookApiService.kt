package net.vpngate.mobile.data.api

import android.content.Context
import android.util.Base64
import android.util.Log
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.withContext
import net.vpngate.mobile.data.model.VpnServer
import okhttp3.OkHttpClient
import okhttp3.Request
import org.json.JSONArray
import org.json.JSONObject
import java.util.concurrent.TimeUnit

class VpnBookApiService {

    companion object {
        private const val TAG = "VpnBookApiService"
        private const val BASE_URL = "https://www.vpnbook.com/freevpn/openvpn"
        private const val CONFIG_API_URL = "https://www.vpnbook.com/api/openvpn"
        private const val USER_AGENT = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0 Safari/537.36"
        @Volatile
        var lastKnownPassword: String? = null
    }

    private val client = OkHttpClient.Builder()
        .connectTimeout(10, TimeUnit.SECONDS)
        .readTimeout(10, TimeUnit.SECONDS)
        .build()

    suspend fun getServers(context: Context): List<VpnServer> = withContext(Dispatchers.IO) {
        try {
            val live = fetchLiveServers()
            if (live.isNotEmpty()) {
                Log.d(TAG, "Successfully fetched ${live.size} live VPNBook servers")
                return@withContext live
            }
        } catch (e: Exception) {
            Log.w(TAG, "Dynamic VPNBook fetch failed (${e.message}), falling back to bundled assets")
        }
        loadBundledServers(context)
    }

    fun loadBundledServers(context: Context): List<VpnServer> {
        return try {
            val jsonStr = context.assets.open("default_vpnbook.json").bufferedReader().use { it.readText() }
            val array = JSONArray(jsonStr)
            val list = mutableListOf<VpnServer>()

            for (i in 0 until array.length()) {
                val obj = array.getJSONObject(i)
                list.add(
                    VpnServer(
                        hostName = obj.optString("host", obj.optString("id")),
                        ip = obj.optString("ip"),
                        score = 800000L,
                        ping = 35L,
                        speed = 100 * 1024 * 1024L,
                        countryLong = obj.optString("countryName"),
                        countryShort = obj.optString("countryCode"),
                        numVpnSessions = 250,
                        uptime = 86400 * 30L,
                        totalUsers = 150000L,
                        totalTraffic = 10000000000L,
                        logType = "none",
                        operator = "VPNBook Dedicated Relay",
                        message = obj.optString("name"),
                        openVpnConfigDataBase64 = obj.optString("configBase64"),
                        source = "vpnbook",
                        protocol = "openvpn",
                        authUsername = obj.optString("username", "vpnbook"),
                        authPassword = lastKnownPassword ?: obj.optString("password", "3ssumf2").takeIf { it != "$" && it.isNotBlank() } ?: "3ssumf2"
                    )
                )
            }
            Log.d(TAG, "Loaded ${list.size} bundled VPNBook servers from assets")
            list
        } catch (e: Exception) {
            Log.e(TAG, "Failed loading bundled VPNBook servers", e)
            emptyList()
        }
    }

    private suspend fun fetchLiveServers(): List<VpnServer> = withContext(Dispatchers.IO) {
        val req = Request.Builder()
            .url(BASE_URL)
            .header("User-Agent", USER_AGENT)
            .header("RSC", "1")
            .build()

        val resp = client.newCall(req).execute()
        if (!resp.isSuccessful) return@withContext emptyList()

        val payload = resp.body?.string() ?: return@withContext emptyList()

        // Extract credentials using precise Next.js RSC children node parsing (matching Desktop Go engine)
        val extractedUser = extractChildValue(payload, "Username")
        val extractedPass = extractChildValue(payload, "Password")
        val username = if (extractedUser.isNotBlank() && extractedUser != "$") extractedUser else "vpnbook"
        val password = if (extractedPass.isNotBlank() && extractedPass != "$") extractedPass else "3ssumf2"
        lastKnownPassword = password

        Log.d(TAG, "VPNBook dynamic credentials extracted: user=$username, pass=${password.take(2)}*** (len=${password.length})")

        // Extract server definitions
        val serverRegex = Regex("""\{"id":"([^"]+)","name":"([^"]+)","hostname":"([^"]+)","ipAddress":"([^"]+)","countryCode":"([^"]+)","countryName":"([^"]+)"\}""")
        val matches = serverRegex.findAll(payload).toList()

        if (matches.isEmpty()) return@withContext emptyList()

        matches.map { match ->
            async {
                val id = match.groupValues[1]
                val name = match.groupValues[2]
                val host = match.groupValues[3]
                val ip = match.groupValues[4]
                val countryCode = match.groupValues[5]
                val countryName = match.groupValues[6]

                try {
                    val cfgReq = Request.Builder()
                        .url("$CONFIG_API_URL?hostname=$host&protocol=tcp443")
                        .header("User-Agent", USER_AGENT)
                        .build()
                    val cfgResp = client.newCall(cfgReq).execute()
                    if (cfgResp.isSuccessful) {
                        val bytes = cfgResp.body?.bytes() ?: return@async null
                        val b64 = Base64.encodeToString(bytes, Base64.NO_WRAP)
                        VpnServer(
                            hostName = host,
                            ip = ip,
                            score = 800000L,
                            ping = 35L,
                            speed = 100 * 1024 * 1024L,
                            countryLong = countryName,
                            countryShort = countryCode,
                            numVpnSessions = 250,
                            uptime = 86400 * 30L,
                            totalUsers = 150000L,
                            totalTraffic = 10000000000L,
                            logType = "none",
                            operator = "VPNBook Dedicated Relay",
                            message = name,
                            openVpnConfigDataBase64 = b64,
                            source = "vpnbook",
                            protocol = "openvpn",
                            authUsername = username,
                            authPassword = password
                        )
                    } else null
                } catch (_: Exception) {
                    null
                }
            }
        }.awaitAll().filterNotNull()
    }

    private fun extractChildValue(payload: String, label: String): String {
        val idx = payload.indexOf(label)
        if (idx < 0) return ""
        val window = payload.substring(idx, (idx + 600).coerceAtMost(payload.length))
        val re = Regex("""children":"([^"]+)"""")
        for (m in re.findAll(window)) {
            val v = m.groupValues[1]
            if (v != label) return v
        }
        return ""
    }
}
