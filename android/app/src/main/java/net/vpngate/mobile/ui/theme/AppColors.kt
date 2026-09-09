package net.vpngate.mobile.ui.theme

import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color

data class AppColors(
    val isDark: Boolean,
    val background: Color,
    val surface: Color,
    val surfaceVariant: Color,
    val border: Color,
    val borderSubtle: Color,
    val textPrimary: Color,
    val textSecondary: Color,
    val textMuted: Color,
    val accentPrimary: Color,
    val accentSecondary: Color,
    val statusConnected: Color,
    val statusConnecting: Color,
    val statusError: Color,
    val pillBackground: Color,
    val navBarBackground: Color,
    val cardShadowColor: Color
)

val DarkAppColors = AppColors(
    isDark = true,
    background = Color(0xFF09090B),
    surface = Color(0xFF18181B),
    surfaceVariant = Color(0xFF202024),
    border = Color(0xFF27272A),
    borderSubtle = Color(0xFF1E1E22),
    textPrimary = Color(0xFFF4F4F5),
    textSecondary = Color(0xFFA1A1AA),
    textMuted = Color(0xFF71717A),
    accentPrimary = Color(0xFF10B981),
    accentSecondary = Color(0xFF06B6D4),
    statusConnected = Color(0xFF34D399),
    statusConnecting = Color(0xFF22D3EE),
    statusError = Color(0xFFF43F5E),
    pillBackground = Color(0xFF18181B),
    navBarBackground = Color(0xFF18181B),
    cardShadowColor = Color.Transparent
)

val LightAppColors = AppColors(
    isDark = false,
    background = Color(0xFFF8FAFC), // Slate 50
    surface = Color(0xFFFFFFFF),    // Pure White cards
    surfaceVariant = Color(0xFFF1F5F9), // Slate 100
    border = Color(0xFFCBD5E1),     // Slate 300
    borderSubtle = Color(0xFFE2E8F0),
    textPrimary = Color(0xFF0F172A), // Slate 900: sharp readability
    textSecondary = Color(0xFF334155), // Slate 700: high contrast secondary
    textMuted = Color(0xFF475569),   // Slate 600: never washed out
    accentPrimary = Color(0xFF059669), // Emerald 600
    accentSecondary = Color(0xFF0284C7), // Sky 600
    statusConnected = Color(0xFF059669),
    statusConnecting = Color(0xFF0284C7),
    statusError = Color(0xFFDC2626),
    pillBackground = Color(0xFFFFFFFF),
    navBarBackground = Color(0xFFFFFFFF),
    cardShadowColor = Color(0x180F172A)
)

val LocalAppColors = staticCompositionLocalOf { DarkAppColors }
