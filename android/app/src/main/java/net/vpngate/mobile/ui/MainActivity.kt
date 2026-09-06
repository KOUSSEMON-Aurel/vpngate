package net.vpngate.mobile.ui

import android.app.Activity
import android.net.VpnService
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.activity.result.contract.ActivityResultContracts
import androidx.activity.viewModels
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.List
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationBarItemDefaults
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import net.vpngate.mobile.ui.screens.HomeScreen
import net.vpngate.mobile.ui.screens.ServersScreen
import net.vpngate.mobile.ui.screens.SettingsScreen
import net.vpngate.mobile.ui.theme.AppTheme
import net.vpngate.mobile.ui.theme.VpnGateTheme
import net.vpngate.mobile.ui.viewmodel.VpnViewModel

class MainActivity : ComponentActivity() {

    private val viewModel: VpnViewModel by viewModels()

    private val vpnPermissionLauncher = registerForActivityResult(
        ActivityResultContracts.StartActivityForResult()
    ) { result ->
        if (result.resultCode == Activity.RESULT_OK) {
            viewModel.selectedServer.value?.let { server ->
                viewModel.connect(this, server) {}
            }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()

        setContent {
            val themeMode by viewModel.themeMode.collectAsState()
            val language by viewModel.language.collectAsState()

            VpnGateTheme(
                themeMode = themeMode,
                appLanguage = language
            ) {
                MainAppScreen(
                    viewModel = viewModel,
                    onRequestVpnPermission = { checkAndRequestVpnPermission() }
                )
            }
        }
    }

    private fun checkAndRequestVpnPermission() {
        val vpnIntent = VpnService.prepare(this)
        if (vpnIntent != null) {
            vpnPermissionLauncher.launch(vpnIntent)
        } else {
            viewModel.selectedServer.value?.let { server ->
                viewModel.connect(this, server) {}
            }
        }
    }
}

sealed class Screen(val route: String) {
    data object Home : Screen("home")
    data object Servers : Screen("servers")
    data object Settings : Screen("settings")
}

data class NavigationTabItem(
    val route: String,
    val title: String,
    val icon: ImageVector
)

@Composable
fun MainAppScreen(
    viewModel: VpnViewModel,
    onRequestVpnPermission: () -> Unit
) {
    val navController = rememberNavController()
    val navBackStackEntry by navController.currentBackStackEntryAsState()
    val currentDestination = navBackStackEntry?.destination

    val colors = AppTheme.colors
    val strings = AppTheme.strings

    val items = listOf(
        NavigationTabItem(Screen.Home.route, strings.tabGateway, Icons.Default.Home),
        NavigationTabItem(Screen.Servers.route, strings.tabRelays, Icons.AutoMirrored.Filled.List),
        NavigationTabItem(Screen.Settings.route, strings.tabSecurity, Icons.Default.Settings)
    )

    Scaffold(
        bottomBar = {
            NavigationBar(
                containerColor = colors.navBarBackground,
                tonalElevation = 0.dp
            ) {
                items.forEach { item ->
                    val isSelected = currentDestination?.route == item.route
                    NavigationBarItem(
                        icon = { Icon(item.icon, contentDescription = item.title) },
                        label = { Text(item.title, fontSize = 11.sp) },
                        selected = isSelected,
                        colors = NavigationBarItemDefaults.colors(
                            selectedIconColor = colors.accentPrimary,
                            selectedTextColor = colors.accentPrimary,
                            unselectedIconColor = colors.textMuted,
                            unselectedTextColor = colors.textMuted,
                            indicatorColor = colors.surfaceVariant
                        ),
                        onClick = {
                            navController.navigate(item.route) {
                                popUpTo(navController.graph.findStartDestination().id) {
                                    saveState = true
                                }
                                launchSingleTop = true
                                restoreState = true
                            }
                        }
                    )
                }
            }
        }
    ) { innerPadding ->
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(colors.background)
                .padding(innerPadding)
        ) {
            NavHost(
                navController = navController,
                startDestination = Screen.Home.route,
                modifier = Modifier.fillMaxSize()
            ) {
                composable(Screen.Home.route) {
                    HomeScreen(
                        viewModel = viewModel,
                        onNavigateToServers = {
                            navController.navigate(Screen.Servers.route)
                        },
                        onRequireVpnPermission = onRequestVpnPermission
                    )
                }
                composable(Screen.Servers.route) {
                    ServersScreen(
                        viewModel = viewModel,
                        onNavigateBack = {
                            navController.popBackStack()
                        },
                        onRequireVpnPermission = onRequestVpnPermission
                    )
                }
                composable(Screen.Settings.route) {
                    SettingsScreen(
                        viewModel = viewModel,
                        onNavigateBack = {
                            navController.popBackStack()
                        }
                    )
                }
            }
        }
    }
}
