package net.vpngate.mobile.data.api

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request
import java.io.IOException
import java.util.concurrent.TimeUnit

class VpnGateApiService {

    private val client = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(20, TimeUnit.SECONDS)
        .followRedirects(true)
        .build()

    private val primaryUrl = "https://www.vpngate.net/api/iphone/"

    suspend fun fetchServerListRaw(): String = withContext(Dispatchers.IO) {
        val request = Request.Builder()
            .url(primaryUrl)
            .header("User-Agent", "vpngate-android/1.0")
            .build()

        client.newCall(request).execute().use { response ->
            if (!response.isSuccessful) {
                throw IOException("HTTP error code: ${response.code}")
            }
            response.body?.string() ?: throw IOException("Empty response body from VPNGate")
        }
    }
}
