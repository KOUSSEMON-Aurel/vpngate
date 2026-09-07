# Proguard rules for OpenRelay VPN
-keepattributes *Annotation*,Signature,InnerClasses,EnclosingMethod

# Keep native methods and JNI bindings
-keepclasseswithmembernames class * {
    native <methods>;
}

# Keep OpenVPN core and JNI bridge
-keep class de.blinkt.openvpn.** { *; }
-keep interface de.blinkt.openvpn.** { *; }

# Keep WireGuard core and JNI bridge
-keep class com.wireguard.android.** { *; }
-keep interface com.wireguard.android.** { *; }

# Keep data models used for state and serialization
-keep class net.vpngate.mobile.data.model.** { *; }
-keep class net.vpngate.mobile.data.prefs.** { *; }

# Keep Coroutines and Flow internals
-keepnames class kotlinx.coroutines.internal.MainDispatcherFactory {}
-keepnames class kotlinx.coroutines.CoroutineExceptionHandler {}

# Annotations
-keepclassmembers class * {
    @org.jetbrains.annotations.* <fields>;
    @org.jetbrains.annotations.* <methods>;
}
