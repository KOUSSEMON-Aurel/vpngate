package net.vpngate.mobile

import android.app.Application
import net.vpngate.mobile.service.UnifiedTunnelManager
import net.vpngate.mobile.service.VpnNotificationManager

class VpnGateApplication : Application() {
    override fun onCreate() {
        super.onCreate()
        VpnNotificationManager.createNotificationChannel(this)
        UnifiedTunnelManager.init(this)
    }
}
