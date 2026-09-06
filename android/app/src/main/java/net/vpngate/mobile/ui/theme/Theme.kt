package net.vpngate.mobile.ui.theme

import android.app.Activity
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.SideEffect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat
import net.vpngate.mobile.data.prefs.AppLanguage
import net.vpngate.mobile.data.prefs.ThemeMode
import net.vpngate.mobile.ui.i18n.AppStrings
import net.vpngate.mobile.ui.i18n.LocalAppStrings
import net.vpngate.mobile.ui.i18n.resolveAppStrings

private val DarkMaterialScheme = darkColorScheme(
    primary = Emerald500,
    onPrimary = Zinc950,
    primaryContainer = Emerald600,
    onPrimaryContainer = Zinc100,
    secondary = Cyan500,
    onSecondary = Zinc950,
    background = Color(0xFF09090B),
    onBackground = Zinc100,
    surface = Color(0xFF18181B),
    onSurface = Zinc100,
    surfaceVariant = Color(0xFF27272A),
    onSurfaceVariant = Zinc200,
    outline = Color(0xFF3F3F46)
)

private val LightMaterialScheme = lightColorScheme(
    primary = Emerald500,
    onPrimary = Color.White,
    primaryContainer = Color(0xFFD1FAE5),
    onPrimaryContainer = Color(0xFF065F46),
    secondary = Cyan500,
    onSecondary = Color.White,
    background = Color(0xFFF8FAFC),
    onBackground = Color(0xFF0F172A),
    surface = Color(0xFFFFFFFF),
    onSurface = Color(0xFF0F172A),
    surfaceVariant = Color(0xFFF1F5F9),
    onSurfaceVariant = Color(0xFF334155),
    outline = Color(0xFFE2E8F0)
)

object AppTheme {
    val colors: AppColors
        @Composable
        @ReadOnlyComposable
        get() = LocalAppColors.current

    val strings: AppStrings
        @Composable
        @ReadOnlyComposable
        get() = LocalAppStrings.current
}

@Composable
fun VpnGateTheme(
    themeMode: ThemeMode = ThemeMode.SYSTEM,
    appLanguage: AppLanguage = AppLanguage.SYSTEM,
    content: @Composable () -> Unit
) {
    val systemInDark = isSystemInDarkTheme()
    val isDark = when (themeMode) {
        ThemeMode.SYSTEM -> systemInDark
        ThemeMode.DARK -> true
        ThemeMode.LIGHT -> false
    }

    val appColors = if (isDark) DarkAppColors else LightAppColors
    val materialScheme = if (isDark) DarkMaterialScheme else LightMaterialScheme
    val appStrings = resolveAppStrings(appLanguage)

    val view = LocalView.current
    if (!view.isInEditMode) {
        SideEffect {
            val window = (view.context as Activity).window
            WindowCompat.getInsetsController(window, view).apply {
                isAppearanceLightStatusBars = !isDark
                isAppearanceLightNavigationBars = !isDark
            }
        }
    }

    CompositionLocalProvider(
        LocalAppColors provides appColors,
        LocalAppStrings provides appStrings
    ) {
        MaterialTheme(
            colorScheme = materialScheme,
            typography = Typography,
            content = content
        )
    }
}
