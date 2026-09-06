package net.vpngate.mobile.data.prefs

import android.content.Context
import android.content.SharedPreferences
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

enum class ThemeMode(val value: String) {
    SYSTEM("system"),
    DARK("dark"),
    LIGHT("light");

    companion object {
        fun fromValue(value: String): ThemeMode =
            entries.find { it.value.equals(value, ignoreCase = true) } ?: SYSTEM
    }
}

enum class AppLanguage(val code: String, val displayName: String, val nativeName: String) {
    SYSTEM("system", "System Default", "Par défaut"),
    FR("fr", "French", "Français"),
    EN("en", "English", "English"),
    ES("es", "Spanish", "Español"),
    DE("de", "German", "Deutsch"),
    JA("ja", "Japanese", "日本語");

    companion object {
        fun fromCode(code: String): AppLanguage =
            entries.find { it.code.equals(code, ignoreCase = true) } ?: SYSTEM
    }
}

enum class ProtocolPreference(val value: String) {
    AUTO("auto"),
    WARP("warp"),
    OPENVPN("openvpn");

    companion object {
        fun fromValue(value: String): ProtocolPreference =
            entries.find { it.value.equals(value, ignoreCase = true) } ?: AUTO
    }
}

class AppPreferences private constructor(context: Context) {

    private val prefs: SharedPreferences =
        context.applicationContext.getSharedPreferences(PREFS_NAME, Context.MODE_PRIVATE)

    private val _themeMode = MutableStateFlow(
        ThemeMode.fromValue(prefs.getString(KEY_THEME_MODE, ThemeMode.SYSTEM.value) ?: ThemeMode.SYSTEM.value)
    )
    val themeMode: StateFlow<ThemeMode> = _themeMode.asStateFlow()

    private val _language = MutableStateFlow(
        AppLanguage.fromCode(prefs.getString(KEY_LANGUAGE, AppLanguage.SYSTEM.code) ?: AppLanguage.SYSTEM.code)
    )
    val language: StateFlow<AppLanguage> = _language.asStateFlow()

    private val _protocolPreference = MutableStateFlow(
        ProtocolPreference.fromValue(prefs.getString(KEY_PROTOCOL, ProtocolPreference.AUTO.value) ?: ProtocolPreference.AUTO.value)
    )
    val protocolPreference: StateFlow<ProtocolPreference> = _protocolPreference.asStateFlow()

    private val _autoReconnect = MutableStateFlow(prefs.getBoolean(KEY_AUTO_RECONNECT, true))
    val autoReconnect: StateFlow<Boolean> = _autoReconnect.asStateFlow()

    private val _dnsProtection = MutableStateFlow(prefs.getBoolean(KEY_DNS_PROTECTION, true))
    val dnsProtection: StateFlow<Boolean> = _dnsProtection.asStateFlow()

    fun setThemeMode(mode: ThemeMode) {
        prefs.edit().putString(KEY_THEME_MODE, mode.value).apply()
        _themeMode.value = mode
    }

    fun setLanguage(lang: AppLanguage) {
        prefs.edit().putString(KEY_LANGUAGE, lang.code).apply()
        _language.value = lang
    }

    fun setProtocolPreference(pref: ProtocolPreference) {
        prefs.edit().putString(KEY_PROTOCOL, pref.value).apply()
        _protocolPreference.value = pref
    }

    fun setAutoReconnect(enabled: Boolean) {
        prefs.edit().putBoolean(KEY_AUTO_RECONNECT, enabled).apply()
        _autoReconnect.value = enabled
    }

    fun setDnsProtection(enabled: Boolean) {
        prefs.edit().putBoolean(KEY_DNS_PROTECTION, enabled).apply()
        _dnsProtection.value = enabled
    }

    companion object {
        private const val PREFS_NAME = "vpngate_app_prefs"
        private const val KEY_THEME_MODE = "theme_mode"
        private const val KEY_LANGUAGE = "app_language"
        private const val KEY_PROTOCOL = "protocol_pref"
        private const val KEY_AUTO_RECONNECT = "auto_reconnect"
        private const val KEY_DNS_PROTECTION = "dns_protection"

        @Volatile
        private var INSTANCE: AppPreferences? = null

        fun getInstance(context: Context): AppPreferences {
            return INSTANCE ?: synchronized(this) {
                INSTANCE ?: AppPreferences(context).also { INSTANCE = it }
            }
        }
    }
}
