package net.vpngate.mobile.ui.i18n

import androidx.compose.runtime.staticCompositionLocalOf
import net.vpngate.mobile.data.prefs.AppLanguage
import java.util.Locale

data class AppStrings(
    val tabGateway: String,
    val tabRelays: String,
    val tabSecurity: String,

    val statusReady: String,
    val statusConnecting: String,
    val statusConnected: String,
    val statusDisconnecting: String,
    val statusFailed: String,

    val selectedLocation: String,
    val connectedLocation: String,
    val selectGateway: String,
    val duration: String,
    val download: String,
    val upload: String,
    val statusLabel: String,
    val statusProtected: String,
    val statusExposed: String,

    val relaysTitle: String,
    val filterAll: String,
    val filterWarp: String,
    val filterOpenVpn: String,
    val searchPlaceholder: String,
    val sortLabel: String,
    val sortPing: String,
    val sortSpeed: String,
    val sortScore: String,
    val btnConnect: String,
    val noServersFound: String,

    val settingsTitle: String,
    val sectionAppearance: String,
    val themeModeLabel: String,
    val themeSystem: String,
    val themeDark: String,
    val themeLight: String,

    val sectionLanguage: String,
    val langSystemDefault: String,

    val sectionSecurity: String,
    val killSwitchTitle: String,
    val killSwitchDesc: String,
    val openAndroidSettings: String,
    val dnsProtectionTitle: String,
    val dnsProtectionDesc: String,

    val sectionData: String,
    val clearCacheTitle: String,
    val clearCacheDesc: String,
    val clearCacheButton: String,
    val cacheClearedToast: String,

    val sectionAbout: String,
    val versionLabel: String,
    val projectDesc: String
)

val EnAppStrings = AppStrings(
    tabGateway = "Gateway",
    tabRelays = "Relays",
    tabSecurity = "Security",

    statusReady = "READY TO CONNECT",
    statusConnecting = "CONNECTING…",
    statusConnected = "CONNECTED",
    statusDisconnecting = "DISCONNECTING…",
    statusFailed = "CONNECTION FAILED",

    selectedLocation = "Selected Location",
    connectedLocation = "Connected Location",
    selectGateway = "Select a Gateway",
    duration = "Duration",
    download = "Download",
    upload = "Upload",
    statusLabel = "Status",
    statusProtected = "Protected",
    statusExposed = "Exposed",

    relaysTitle = "Relay Servers",
    filterAll = "All Relays",
    filterWarp = "WARP",
    filterOpenVpn = "OpenVPN",
    searchPlaceholder = "Search country, IP, or node…",
    sortLabel = "Sort:",
    sortPing = "Ping",
    sortSpeed = "Speed",
    sortScore = "Score",
    btnConnect = "Connect",
    noServersFound = "No VPN servers found matching filters",

    settingsTitle = "Settings & Security",
    sectionAppearance = "Appearance",
    themeModeLabel = "Theme Mode",
    themeSystem = "System",
    themeDark = "Dark",
    themeLight = "Light",

    sectionLanguage = "Language",
    langSystemDefault = "System Default",

    sectionSecurity = "Network & Security",
    killSwitchTitle = "System Kill Switch",
    killSwitchDesc = "Block all non-VPN traffic directly in Android settings",
    openAndroidSettings = "Open Android Settings",
    dnsProtectionTitle = "DNS Leak Protection",
    dnsProtectionDesc = "Strict encrypted resolver via Cloudflare 1.1.1.1",

    sectionData = "Relay Data & Cache",
    clearCacheTitle = "Reset Relay Cache",
    clearCacheDesc = "Purge cached relays and fetch fresh list from VPNGate",
    clearCacheButton = "Clear Cache",
    cacheClearedToast = "Relay cache cleared",

    sectionAbout = "About VPNGate Mobile",
    versionLabel = "Version",
    projectDesc = "Open-source academic VPN project hosted by University of Tsukuba, Japan."
)

val FrAppStrings = AppStrings(
    tabGateway = "Passerelle",
    tabRelays = "Relais",
    tabSecurity = "Sécurité",

    statusReady = "PRÊT À CONNECTER",
    statusConnecting = "CONNEXION…",
    statusConnected = "CONNECTÉ",
    statusDisconnecting = "DÉCONNEXION…",
    statusFailed = "ÉCHEC DE CONNEXION",

    selectedLocation = "Emplacement sélectionné",
    connectedLocation = "Emplacement connecté",
    selectGateway = "Sélectionner une passerelle",
    duration = "Durée",
    download = "Téléchargement",
    upload = "Envoi",
    statusLabel = "Statut",
    statusProtected = "Protégé",
    statusExposed = "Exposé",

    relaysTitle = "Serveurs Relais",
    filterAll = "Tous les relais",
    filterWarp = "WARP",
    filterOpenVpn = "OpenVPN",
    searchPlaceholder = "Rechercher pays, IP ou nœud…",
    sortLabel = "Trier :",
    sortPing = "Ping",
    sortSpeed = "Vitesse",
    sortScore = "Score",
    btnConnect = "Connexion",
    noServersFound = "Aucun serveur VPN ne correspond aux filtres",

    settingsTitle = "Paramètres & Sécurité",
    sectionAppearance = "Apparence",
    themeModeLabel = "Thème d'affichage",
    themeSystem = "Système",
    themeDark = "Sombre",
    themeLight = "Clair",

    sectionLanguage = "Langue",
    langSystemDefault = "Langue du système",

    sectionSecurity = "Réseau & Sécurité",
    killSwitchTitle = "Kill Switch Système",
    killSwitchDesc = "Bloque toutes les connexions sans VPN dans les paramètres Android",
    openAndroidSettings = "Ouvrir les paramètres Android",
    dnsProtectionTitle = "Protection Fuite DNS",
    dnsProtectionDesc = "Résolveur sécurisé chiffré via Cloudflare 1.1.1.1",

    sectionData = "Données & Cache",
    clearCacheTitle = "Vider le cache des relais",
    clearCacheDesc = "Supprime les serveurs en cache et recharge la liste officielle",
    clearCacheButton = "Vider le cache",
    cacheClearedToast = "Cache des relais vidé avec succès",

    sectionAbout = "À propos de VPNGate",
    versionLabel = "Version",
    projectDesc = "Projet VPN académique libre hébergé par l'Université de Tsukuba, Japon."
)

val EsAppStrings = AppStrings(
    tabGateway = "Pasarela",
    tabRelays = "Relés",
    tabSecurity = "Seguridad",

    statusReady = "LISTO PARA CONECTAR",
    statusConnecting = "CONECTANDO…",
    statusConnected = "CONECTADO",
    statusDisconnecting = "DESCONECTANDO…",
    statusFailed = "ERROR DE CONEXIÓN",

    selectedLocation = "Ubicación seleccionada",
    connectedLocation = "Ubicación conectada",
    selectGateway = "Seleccionar pasarela",
    duration = "Duración",
    download = "Descarga",
    upload = "Subida",
    statusLabel = "Estado",
    statusProtected = "Protegido",
    statusExposed = "Expuesto",

    relaysTitle = "Servidores Relay",
    filterAll = "Todos los relés",
    filterWarp = "WARP",
    filterOpenVpn = "OpenVPN",
    searchPlaceholder = "Buscar país, IP o nodo…",
    sortLabel = "Ordenar:",
    sortPing = "Ping",
    sortSpeed = "Velocidad",
    sortScore = "Puntuación",
    btnConnect = "Conectar",
    noServersFound = "No se encontraron servidores VPN",

    settingsTitle = "Ajustes y Seguridad",
    sectionAppearance = "Apariencia",
    themeModeLabel = "Modo de tema",
    themeSystem = "Sistema",
    themeDark = "Oscuro",
    themeLight = "Claro",

    sectionLanguage = "Idioma",
    langSystemDefault = "Predeterminado del sistema",

    sectionSecurity = "Red y Seguridad",
    killSwitchTitle = "Kill Switch del Sistema",
    killSwitchDesc = "Bloquear todo el tráfico no VPN en los ajustes de Android",
    openAndroidSettings = "Abrir ajustes de Android",
    dnsProtectionTitle = "Protección Fugas DNS",
    dnsProtectionDesc = "DNS cifrado estricto mediante Cloudflare 1.1.1.1",

    sectionData = "Datos y Caché",
    clearCacheTitle = "Restablecer caché de servidores",
    clearCacheDesc = "Purgar servidores guardados y obtener la lista fresca",
    clearCacheButton = "Borrar caché",
    cacheClearedToast = "Caché de servidores borrada",

    sectionAbout = "Acerca de VPNGate Mobile",
    versionLabel = "Versión",
    projectDesc = "Proyecto VPN académico de código abierto de la Univ. de Tsukuba, Japón."
)

val DeAppStrings = AppStrings(
    tabGateway = "Gateway",
    tabRelays = "Relais",
    tabSecurity = "Sicherheit",

    statusReady = "BEREIT ZUM VERBINDEN",
    statusConnecting = "VERBINDEN…",
    statusConnected = "VERBUNDEN",
    statusDisconnecting = "TRENNEN…",
    statusFailed = "VERBINDUNG FEHLGESCHLAGEN",

    selectedLocation = "Ausgewählter Standort",
    connectedLocation = "Verbundener Standort",
    selectGateway = "Gateway auswählen",
    duration = "Dauer",
    download = "Download",
    upload = "Upload",
    statusLabel = "Status",
    statusProtected = "Geschützt",
    statusExposed = "Ungeschützt",

    relaysTitle = "Relay-Server",
    filterAll = "Alle Relais",
    filterWarp = "WARP",
    filterOpenVpn = "OpenVPN",
    searchPlaceholder = "Land, IP oder Node suchen…",
    sortLabel = "Sortieren:",
    sortPing = "Ping",
    sortSpeed = "Geschwindigkeit",
    sortScore = "Score",
    btnConnect = "Verbinden",
    noServersFound = "Keine VPN-Server gefunden",

    settingsTitle = "Einstellungen & Sicherheit",
    sectionAppearance = "Erscheinungsbild",
    themeModeLabel = "Design-Modus",
    themeSystem = "System",
    themeDark = "Dunkel",
    themeLight = "Hell",

    sectionLanguage = "Sprache",
    langSystemDefault = "Systemstandard",

    sectionSecurity = "Netzwerk & Sicherheit",
    killSwitchTitle = "System-Kill-Switch",
    killSwitchDesc = "Alle Verbindungen ohne VPN in Android-Einstellungen blockieren",
    openAndroidSettings = "Android-Einstellungen öffnen",
    dnsProtectionTitle = "DNS-Leak-Schutz",
    dnsProtectionDesc = "Strikt verschlüsselter DNS-Resolver über Cloudflare 1.1.1.1",

    sectionData = "Relay-Daten & Cache",
    clearCacheTitle = "Relay-Cache leeren",
    clearCacheDesc = "Gespeicherte Server löschen und neue Liste laden",
    clearCacheButton = "Cache leeren",
    cacheClearedToast = "Relay-Cache erfolgreich geleert",

    sectionAbout = "Über VPNGate Mobile",
    versionLabel = "Version",
    projectDesc = "Open-Source-VPN-Projekt der Universität Tsukuba, Japan."
)

val JaAppStrings = AppStrings(
    tabGateway = "ゲートウェイ",
    tabRelays = "リレー",
    tabSecurity = "セキュリティ",

    statusReady = "接続待機中",
    statusConnecting = "接続中…",
    statusConnected = "接続完了",
    statusDisconnecting = "切断中…",
    statusFailed = "接続失敗",

    selectedLocation = "選択されたサーバー",
    connectedLocation = "接続中サーバー",
    selectGateway = "ゲートウェイを選択",
    duration = "接続時間",
    download = "受信",
    upload = "送信",
    statusLabel = "保護状態",
    statusProtected = "保護中",
    statusExposed = "非保護",

    relaysTitle = "リレーサーバー一覧",
    filterAll = "すべて",
    filterWarp = "WARP",
    filterOpenVpn = "OpenVPN",
    searchPlaceholder = "国、IPアドレス、ノードを検索…",
    sortLabel = "並び替え:",
    sortPing = "Ping",
    sortSpeed = "速度",
    sortScore = "スコア",
    btnConnect = "接続",
    noServersFound = "該当するVPNサーバーが見つかりません",

    settingsTitle = "設定とセキュリティ",
    sectionAppearance = "外観",
    themeModeLabel = "テーマモード",
    themeSystem = "システム既定",
    themeDark = "ダーク",
    themeLight = "ライト",

    sectionLanguage = "言語",
    langSystemDefault = "端末の言語",

    sectionSecurity = "ネットワークとセキュリティ",
    killSwitchTitle = "システムキルスイッチ",
    killSwitchDesc = "VPN未接続時のインターネット通信を遮断",
    openAndroidSettings = "Androidの設定を開く",
    dnsProtectionTitle = "DNS漏洩保護",
    dnsProtectionDesc = "Cloudflare 1.1.1.1 による暗号化DNS",

    sectionData = "リレーデータとキャッシュ",
    clearCacheTitle = "リレーキャッシュの初期化",
    clearCacheDesc = "保存済みサーバーを破棄し、VPNGateから最新リストを再取得",
    clearCacheButton = "キャッシュを消去",
    cacheClearedToast = "リレーキャッシュを消去しました",

    sectionAbout = "VPNGate Mobileについて",
    versionLabel = "バージョン",
    projectDesc = "日本の筑波大学による学術的オープンソースVPNプロジェクト。"
)

fun resolveAppStrings(language: AppLanguage): AppStrings {
    return when (language) {
        AppLanguage.EN -> EnAppStrings
        AppLanguage.FR -> FrAppStrings
        AppLanguage.ES -> EsAppStrings
        AppLanguage.DE -> DeAppStrings
        AppLanguage.JA -> JaAppStrings
        AppLanguage.SYSTEM -> {
            val systemLang = Locale.getDefault().language.lowercase()
            when {
                systemLang.startsWith("fr") -> FrAppStrings
                systemLang.startsWith("es") -> EsAppStrings
                systemLang.startsWith("de") -> DeAppStrings
                systemLang.startsWith("ja") -> JaAppStrings
                else -> EnAppStrings
            }
        }
    }
}

val LocalAppStrings = staticCompositionLocalOf { EnAppStrings }
