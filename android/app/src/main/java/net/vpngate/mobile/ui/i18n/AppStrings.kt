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
    val projectDesc: String,

    val vpnDisclosureTitle: String,
    val vpnDisclosureBody: String,
    val vpnDisclosureAccept: String,
    val vpnDisclosureDecline: String,
    val openSourceLicenses: String,
    val privacyPolicy: String,
    val academicDisclaimer: String
)

val EnAppStrings = AppStrings(
    tabGateway = "Gateway",
    tabRelays = "Relays",
    tabSecurity = "Security",

    statusReady = "READY TO CONNECT",
    statusConnecting = "CONNECTING…",
    statusConnected = "CONNECTED",
    statusDisconnecting = "DISCONNECTING…",
    statusFailed = "FAILED",

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
    filterWarp = "WireGuard",
    filterOpenVpn = "OpenVPN",
    searchPlaceholder = "Search country, IP, or node...",
    sortLabel = "Sort:",
    sortPing = "Ping",
    sortSpeed = "Speed",
    sortScore = "Score",
    btnConnect = "Connect",
    noServersFound = "No servers found matching your query.",

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
    dnsProtectionDesc = "Strict encrypted resolver via Privacy DNS (1.1.1.1)",

    sectionData = "Relay Data & Cache",
    clearCacheTitle = "Reset Relay Cache",
    clearCacheDesc = "Purge cached relays and fetch fresh list from VPN Gate",
    clearCacheButton = "Clear Cache",
    cacheClearedToast = "Cache cleared, refreshing relays...",

    sectionAbout = "About OpenRelay VPN",
    versionLabel = "Version 2.2.0",
    projectDesc = "Open-source academic VPN client for the VPN Gate project hosted by University of Tsukuba, Japan.",

    vpnDisclosureTitle = "VPN Service & Privacy Disclosure",
    vpnDisclosureBody = "OpenRelay VPN uses Android's VpnService to route and encrypt your internet traffic through community VPN Gate academic relays. No personal data or browsing logs are collected, tracked, or sold by this application.",
    vpnDisclosureAccept = "Accept & Continue",
    vpnDisclosureDecline = "Decline",
    openSourceLicenses = "Open Source Licenses",
    privacyPolicy = "Privacy Policy",
    academicDisclaimer = "OpenRelay VPN is an independent open-source client. Not officially affiliated with or endorsed by University of Tsukuba."
)

val FrAppStrings = AppStrings(
    tabGateway = "Gateway",
    tabRelays = "Relais",
    tabSecurity = "Sécurité",

    statusReady = "PRÊT À CONNECTER",
    statusConnecting = "CONNEXION EN COURS…",
    statusConnected = "CONNECTÉ",
    statusDisconnecting = "DÉCONNEXION…",
    statusFailed = "ÉCHEC",

    selectedLocation = "Emplacement sélectionné",
    connectedLocation = "Emplacement connecté",
    selectGateway = "Sélectionner un relais",
    duration = "Durée",
    download = "Descendant",
    upload = "Montant",
    statusLabel = "Statut",
    statusProtected = "Protégé",
    statusExposed = "Exposé",

    relaysTitle = "Serveurs Relais",
    filterAll = "Tous les relais",
    filterWarp = "WireGuard",
    filterOpenVpn = "OpenVPN",
    searchPlaceholder = "Chercher pays, IP ou nœud...",
    sortLabel = "Trier :",
    sortPing = "Ping",
    sortSpeed = "Débit",
    sortScore = "Score",
    btnConnect = "Connecter",
    noServersFound = "Aucun serveur ne correspond à votre recherche.",

    settingsTitle = "Paramètres & Sécurité",
    sectionAppearance = "Apparence",
    themeModeLabel = "Mode Thème",
    themeSystem = "Système",
    themeDark = "Sombre",
    themeLight = "Clair",

    sectionLanguage = "Langue",
    langSystemDefault = "Langue du système",

    sectionSecurity = "Réseau & Sécurité",
    killSwitchTitle = "Kill Switch Système",
    killSwitchDesc = "Bloquer tout trafic hors VPN dans les paramètres Android",
    openAndroidSettings = "Ouvrir Paramètres Android",
    dnsProtectionTitle = "Protection contre les fuites DNS",
    dnsProtectionDesc = "Résolveur sécurisé chiffré via DNS sécurisé (1.1.1.1)",

    sectionData = "Cache & Données Relais",
    clearCacheTitle = "Réinitialiser le Cache",
    clearCacheDesc = "Purger les relais en cache et recharger la liste VPN Gate",
    clearCacheButton = "Vider le Cache",
    cacheClearedToast = "Cache purgé, rechargement en cours...",

    sectionAbout = "À propos de OpenRelay VPN",
    versionLabel = "Version 2.2.0",
    projectDesc = "Client VPN académique open-source pour le projet VPN Gate hébergé par l'Université de Tsukuba, Japon.",

    vpnDisclosureTitle = "Service VPN & Confidentialité",
    vpnDisclosureBody = "OpenRelay VPN utilise le VpnService d'Android pour acheminer et chiffrer votre trafic Internet à travers les relais académiques VPN Gate. Aucune donnée personnelle ni journal de navigation n'est collecté, suivi ou vendu par cette application.",
    vpnDisclosureAccept = "Accepter et Continuer",
    vpnDisclosureDecline = "Refuser",
    openSourceLicenses = "Licences Open Source",
    privacyPolicy = "Politique de confidentialité",
    academicDisclaimer = "OpenRelay VPN est un client open-source indépendant. Non affilié officiellement à l'Université de Tsukuba."
)

val EsAppStrings = AppStrings(
    tabGateway = "Gateway",
    tabRelays = "Servidores",
    tabSecurity = "Seguridad",

    statusReady = "LISTO PARA CONECTAR",
    statusConnecting = "CONECTANDO…",
    statusConnected = "CONECTADO",
    statusDisconnecting = "DESCONECTANDO…",
    statusFailed = "ERROR",

    selectedLocation = "Ubicación seleccionada",
    connectedLocation = "Ubicación conectada",
    selectGateway = "Seleccionar un servidor",
    duration = "Duración",
    download = "Bajada",
    upload = "Subida",
    statusLabel = "Estado",
    statusProtected = "Protegido",
    statusExposed = "Expuesto",

    relaysTitle = "Servidores Relé",
    filterAll = "Todos",
    filterWarp = "WireGuard",
    filterOpenVpn = "OpenVPN",
    searchPlaceholder = "Buscar país, IP o nodo...",
    sortLabel = "Ordenar:",
    sortPing = "Ping",
    sortSpeed = "Velocidad",
    sortScore = "Puntuación",
    btnConnect = "Conectar",
    noServersFound = "No se encontraron servidores.",

    settingsTitle = "Ajustes y Seguridad",
    sectionAppearance = "Apariencia",
    themeModeLabel = "Tema",
    themeSystem = "Sistema",
    themeDark = "Oscuro",
    themeLight = "Claro",

    sectionLanguage = "Idioma",
    langSystemDefault = "Predeterminado del sistema",

    sectionSecurity = "Red y Seguridad",
    killSwitchTitle = "Kill Switch del Sistema",
    killSwitchDesc = "Bloquear tráfico no VPN directamente en ajustes de Android",
    openAndroidSettings = "Abrir Ajustes de Android",
    dnsProtectionTitle = "Protección contra fugas DNS",
    dnsProtectionDesc = "DNS cifrado estricto mediante resolver seguro (1.1.1.1)",

    sectionData = "Caché de Servidores",
    clearCacheTitle = "Restablecer Caché",
    clearCacheDesc = "Vaciar la caché y descargar lista fresca de VPN Gate",
    clearCacheButton = "Borrar Caché",
    cacheClearedToast = "Caché eliminada, recargando...",

    sectionAbout = "Acerca de OpenRelay VPN",
    versionLabel = "Versión 2.2.0",
    projectDesc = "Cliente VPN académico de código abierto para el proyecto VPN Gate de la Universidad de Tsukuba, Japón.",

    vpnDisclosureTitle = "Servicio VPN y Privacidad",
    vpnDisclosureBody = "OpenRelay VPN utiliza VpnService de Android para enrutar y cifrar su tráfico a través de repetidores académicos de VPN Gate. Esta aplicación no recopila, rastrea ni vende sus datos personales ni registros de navegación.",
    vpnDisclosureAccept = "Aceptar y Continuar",
    vpnDisclosureDecline = "Rechazar",
    openSourceLicenses = "Licencias de código abierto",
    privacyPolicy = "Política de privacidad",
    academicDisclaimer = "OpenRelay VPN es un cliente de código abierto independiente. No está afiliado oficialmente a la Universidad de Tsukuba."
)

val DeAppStrings = AppStrings(
    tabGateway = "Gateway",
    tabRelays = "Relais",
    tabSecurity = "Sicherheit",

    statusReady = "BEREIT ZUM VERBINDEN",
    statusConnecting = "VERBINDUNG WIRD HERGESTELLT…",
    statusConnected = "VERBUNDEN",
    statusDisconnecting = "TRENNEN…",
    statusFailed = "FEHLGESCHLAGEN",

    selectedLocation = "Ausgewählter Standort",
    connectedLocation = "Verbundener Standort",
    selectGateway = "Relais auswählen",
    duration = "Dauer",
    download = "Download",
    upload = "Upload",
    statusLabel = "Status",
    statusProtected = "Geschützt",
    statusExposed = "Ungeschützt",

    relaysTitle = "Relais-Server",
    filterAll = "Alle Relais",
    filterWarp = "WireGuard",
    filterOpenVpn = "OpenVPN",
    searchPlaceholder = "Land, IP oder Knoten suchen...",
    sortLabel = "Sortieren:",
    sortPing = "Ping",
    sortSpeed = "Geschwindigkeit",
    sortScore = "Score",
    btnConnect = "Verbinden",
    noServersFound = "Keine passenden Server gefunden.",

    settingsTitle = "Einstellungen & Sicherheit",
    sectionAppearance = "Erscheinungsbild",
    themeModeLabel = "Designmodus",
    themeSystem = "System",
    themeDark = "Dunkel",
    themeLight = "Hell",

    sectionLanguage = "Sprache",
    langSystemDefault = "Systemstandard",

    sectionSecurity = "Netzwerk & Sicherheit",
    killSwitchTitle = "System-Kill-Switch",
    killSwitchDesc = "Nicht-VPN-Verkehr direkt in den Android-Einstellungen blockieren",
    openAndroidSettings = "Android-Einstellungen öffnen",
    dnsProtectionTitle = "DNS-Leckschutz",
    dnsProtectionDesc = "Strikt verschlüsselter DNS-Resolver über sicheres DNS (1.1.1.1)",

    sectionData = "Relais-Cache",
    clearCacheTitle = "Relais-Cache zurücksetzen",
    clearCacheDesc = "Zwischengespeicherte Relais leeren und neue Liste von VPN Gate laden",
    clearCacheButton = "Cache leeren",
    cacheClearedToast = "Cache geleert, Relais werden aktualisiert...",

    sectionAbout = "Über OpenRelay VPN",
    versionLabel = "Version 2.2.0",
    projectDesc = "Open-Source-VPN-Client für das akademische VPN Gate-Projekt der Universität Tsukuba, Japan.",

    vpnDisclosureTitle = "VPN-Dienst & Datenschutzhinweis",
    vpnDisclosureBody = "OpenRelay VPN verwendet den Android VpnService, um Ihren Datenverkehr über akademische VPN Gate-Relais zu leiten und zu verschlüsseln. Diese Anwendung sammelt, verfolgt oder verkauft keine persönlichen Daten oder Surfprotokolle.",
    vpnDisclosureAccept = "Akzeptieren & Weiter",
    vpnDisclosureDecline = "Ablehnen",
    openSourceLicenses = "Open-Source-Lizenzen",
    privacyPolicy = "Datenschutzerklärung",
    academicDisclaimer = "OpenRelay VPN ist ein unabhängiger Open-Source-Client. Nicht offiziell mit der Universität Tsukuba verbunden."
)

val JaAppStrings = AppStrings(
    tabGateway = "Gateway",
    tabRelays = "中継サーバー",
    tabSecurity = "セキュリティ",

    statusReady = "接続待機中",
    statusConnecting = "接続中…",
    statusConnected = "接続完了",
    statusDisconnecting = "切断中…",
    statusFailed = "接続失敗",

    selectedLocation = "選択中のサーバー",
    connectedLocation = "接続中のサーバー",
    selectGateway = "サーバーを選択",
    duration = "接続時間",
    download = "受信",
    upload = "送信",
    statusLabel = "保護状態",
    statusProtected = "保護中",
    statusExposed = "未保護",

    relaysTitle = "中継サーバー一覧",
    filterAll = "すべて",
    filterWarp = "WireGuard",
    filterOpenVpn = "OpenVPN",
    searchPlaceholder = "国、IP、ホスト名で検索...",
    sortLabel = "並び替え:",
    sortPing = "Ping",
    sortSpeed = "速度",
    sortScore = "スコア",
    btnConnect = "接続する",
    noServersFound = "条件に一致するサーバーが見つかりませんでした。",

    settingsTitle = "設定とセキュリティ",
    sectionAppearance = "外観",
    themeModeLabel = "テーマ",
    themeSystem = "システム準拠",
    themeDark = "ダーク",
    themeLight = "ライト",

    sectionLanguage = "言語設定",
    langSystemDefault = "端末の標準言語",

    sectionSecurity = "ネットワークとセキュリティ",
    killSwitchTitle = "システムキルスイッチ",
    killSwitchDesc = "VPN以外の通信をAndroidシステム設定で完全に遮断します",
    openAndroidSettings = "Android設定を開く",
    dnsProtectionTitle = "DNSリーク保護",
    dnsProtectionDesc = "暗号化プライベートDNS (1.1.1.1) による保護",

    sectionData = "サーバーキャッシュ管理",
    clearCacheTitle = "キャッシュの消去",
    clearCacheDesc = "保存されたサーバーリストを消去し、VPN Gateから最新データを再取得します",
    clearCacheButton = "キャッシュ消去",
    cacheClearedToast = "キャッシュを消去しました。再取得中...",

    sectionAbout = "OpenRelay VPN について",
    versionLabel = "バージョン 2.2.0",
    projectDesc = "日本の筑波大学による学術実験プロジェクト VPN Gate に接続するためのオープンソースクライアントです。",

    vpnDisclosureTitle = "VPNサービスとプライバシーの開示",
    vpnDisclosureBody = "OpenRelay VPNは、AndroidのVpnServiceを使用して、VPN Gate学術中継サーバー経由で通信を暗号化・ルーティングします。本アプリが個人情報や閲覧ログを収集・追跡・販売することはありません。",
    vpnDisclosureAccept = "同意して続行",
    vpnDisclosureDecline = "辞退",
    openSourceLicenses = "オープンソースライセンス",
    privacyPolicy = "プライバシーポリシー",
    academicDisclaimer = "OpenRelay VPNは独立したオープンソースクライアントであり、筑波大学の公式認定を受けたものではありません。"
)

fun resolveAppStrings(language: AppLanguage): AppStrings {
    return when (language) {
        AppLanguage.EN -> EnAppStrings
        AppLanguage.FR -> FrAppStrings
        AppLanguage.ES -> EsAppStrings
        AppLanguage.DE -> DeAppStrings
        AppLanguage.JA -> JaAppStrings
        AppLanguage.SYSTEM -> {
            val sysLang = Locale.getDefault().language.lowercase()
            when {
                sysLang.startsWith("fr") -> FrAppStrings
                sysLang.startsWith("es") -> EsAppStrings
                sysLang.startsWith("de") -> DeAppStrings
                sysLang.startsWith("ja") -> JaAppStrings
                else -> EnAppStrings
            }
        }
    }
}

val LocalAppStrings = staticCompositionLocalOf { EnAppStrings }
