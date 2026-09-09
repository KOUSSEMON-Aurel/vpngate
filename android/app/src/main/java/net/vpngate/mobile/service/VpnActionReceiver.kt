package net.vpngate.mobile.service

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.util.Log

class VpnActionReceiver : BroadcastReceiver() {

    companion object {
        const val ACTION_DISCONNECT_VPN = "net.vpngate.mobile.ACTION_DISCONNECT_VPN"
    }

    override fun onReceive(context: Context, intent: Intent?) {
        if (intent?.action == ACTION_DISCONNECT_VPN) {
            Log.d("VpnActionReceiver", "Disconnect action received from notification. Stopping VPN...")
            UnifiedTunnelManager.stopVpn(context.applicationContext)
        }
    }
}
