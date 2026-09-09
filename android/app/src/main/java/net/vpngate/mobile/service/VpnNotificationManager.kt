package net.vpngate.mobile.service

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.os.Build
import android.util.Log
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import net.vpngate.mobile.R
import net.vpngate.mobile.data.model.VpnServer
import net.vpngate.mobile.ui.MainActivity

object VpnNotificationManager {

    private const val TAG = "VpnNotificationManager"
    const val CHANNEL_ID = "openrelay_vpn_status"
    private const val NOTIFICATION_ID = 1001

    fun createNotificationChannel(context: Context) {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val name = "Statut OpenRelay VPN"
            val descriptionText = "Affiche l'état de la connexion VPN et permet la déconnexion directe"
            val importance = NotificationManager.IMPORTANCE_LOW
            val channel = NotificationChannel(CHANNEL_ID, name, importance).apply {
                description = descriptionText
                setShowBadge(false)
            }
            val notificationManager = context.getSystemService(Context.NOTIFICATION_SERVICE) as NotificationManager
            notificationManager.createNotificationChannel(channel)
        }
    }

    fun showConnecting(context: Context, server: VpnServer?) {
        createNotificationChannel(context)
        val location = server?.countryLong ?: "Relais VPN"
        buildAndNotify(
            context = context,
            title = "OpenRelay VPN • Connexion en cours",
            message = "Établissement du tunnel chiffré vers $location...",
            showDisconnect = true
        )
    }

    fun showConnected(context: Context, server: VpnServer?, detectedIp: String?) {
        createNotificationChannel(context)
        val location = server?.countryLong ?: "Relais sécurisé"
        val ipInfo = if (!detectedIp.isNullOrBlank()) " ($detectedIp)" else ""
        val protoInfo = if (server?.isWarp == true) "WireGuard Anycast" else "OpenVPN Relais"
        buildAndNotify(
            context = context,
            title = "OpenRelay VPN • Connecté",
            message = "$location$ipInfo • $protoInfo actif",
            showDisconnect = true
        )
    }

    fun dismiss(context: Context) {
        try {
            val notificationManager = NotificationManagerCompat.from(context)
            notificationManager.cancel(NOTIFICATION_ID)
            Log.d(TAG, "Notification dismissed")
        } catch (e: Exception) {
            Log.w(TAG, "Failed to dismiss notification: ${e.message}")
        }
    }

    private fun buildAndNotify(
        context: Context,
        title: String,
        message: String,
        showDisconnect: Boolean
    ) {
        try {
            // Intent to open MainActivity on click
            val openAppIntent = Intent(context, MainActivity::class.java).apply {
                flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            }
            val openAppPendingIntent = PendingIntent.getActivity(
                context,
                0,
                openAppIntent,
                PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
            )

            val builder = NotificationCompat.Builder(context, CHANNEL_ID)
                .setSmallIcon(R.drawable.ic_vpn_key)
                .setContentTitle(title)
                .setContentText(message)
                .setStyle(NotificationCompat.BigTextStyle().bigText(message))
                .setPriority(NotificationCompat.PRIORITY_LOW)
                .setOngoing(true)
                .setAutoCancel(false)
                .setContentIntent(openAppPendingIntent)

            if (showDisconnect) {
                // Action to disconnect directly from notification
                val disconnectIntent = Intent(context, VpnActionReceiver::class.java).apply {
                    action = VpnActionReceiver.ACTION_DISCONNECT_VPN
                }
                val disconnectPendingIntent = PendingIntent.getBroadcast(
                    context,
                    1,
                    disconnectIntent,
                    PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE
                )
                builder.addAction(
                    android.R.drawable.ic_menu_close_clear_cancel,
                    "Déconnecter",
                    disconnectPendingIntent
                )
            }

            val notificationManager = NotificationManagerCompat.from(context)
            notificationManager.notify(NOTIFICATION_ID, builder.build())
            Log.d(TAG, "Notification updated: $title")
        } catch (e: SecurityException) {
            Log.w(TAG, "Notification permission not granted: ${e.message}")
        } catch (e: Exception) {
            Log.e(TAG, "Failed to post notification", e)
        }
    }
}
