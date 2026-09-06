package net.vpngate.mobile.data.api

import android.util.Log
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.OkHttpClient
import okhttp3.Request
import java.io.IOException
import java.util.concurrent.TimeUnit

class VpnGateApiService {

    private val client = OkHttpClient.Builder()
        .connectTimeout(45, TimeUnit.SECONDS)
        .readTimeout(60, TimeUnit.SECONDS)
        .followRedirects(true)
        .build()

    private val urls = listOf(
        "https://www.vpngate.net/api/iphone/",
        "http://www.vpngate.net/api/iphone/",
        "http://219.100.37.243/api/iphone/",
        "http://130.158.6.81/api/iphone/"
    )

    suspend fun fetchServerListRaw(): String = withContext(Dispatchers.IO) {
        var lastException: Exception? = null

        for (url in urls) {
            try {
                Log.d("VPNGate", "Fetching server list from $url...")
                val request = Request.Builder()
                    .url(url)
                    .header("User-Agent", "Mozilla/5.0 (Android; Mobile; rv:109.0) Gecko/119.0 Firefox/119.0")
                    .build()

                val content = client.newCall(request).execute().use { response ->
                    if (!response.isSuccessful) {
                        throw IOException("HTTP error code: ${response.code} from $url")
                    }
                    response.body?.string() ?: throw IOException("Empty response body from $url")
                }

                Log.d("VPNGate", "Successfully received ${content.length} characters from $url")
                return@withContext content
            } catch (e: Exception) {
                Log.w("VPNGate", "Failed fetching from $url: ${e.message}")
                lastException = e
            }
        }

        throw lastException ?: IOException("Failed to fetch from all VPNGate endpoints")
    }
}
