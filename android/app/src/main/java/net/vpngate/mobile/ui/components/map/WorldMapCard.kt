package net.vpngate.mobile.ui.components.map

import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CenterFocusStrong
import androidx.compose.material.icons.filled.Public
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import net.vpngate.mobile.R
import net.vpngate.mobile.data.model.ConnectionStatus
import net.vpngate.mobile.data.model.VpnServer
import net.vpngate.mobile.ui.theme.AppTheme
import kotlin.math.hypot
import kotlin.math.min

private val ArcDashIntervals = floatArrayOf(16f, 10f)

data class MapCountryNode(
    val countryCode: String,
    val countryName: String,
    val serverCount: Int,
    val bestPing: Long,
    val normalizedPos: Offset,
    val representativeServer: VpnServer
)

@Composable
fun WorldMapCard(
    servers: List<VpnServer>,
    selectedServer: VpnServer?,
    connectedServer: VpnServer?,
    status: ConnectionStatus,
    onSelectServer: (VpnServer) -> Unit,
    modifier: Modifier = Modifier
) {
    val colors = AppTheme.colors
    val strings = AppTheme.strings

    val activeServer = if (status == ConnectionStatus.CONNECTED) connectedServer ?: selectedServer else selectedServer
    val activeCountryCode = (activeServer?.countryShort ?: "JP").uppercase()

    // Aggregate servers by country
    val countryNodes = remember(servers) {
        servers.filter { !it.isWarp && it.countryShort.isNotBlank() }
            .groupBy { it.countryShort.uppercase() }
            .mapNotNull { (code, sList) ->
                val norm = CountryCoordinates.getNormalizedOffset(code) ?: return@mapNotNull null
                val best = sList.minByOrNull { it.ping } ?: sList.first()
                MapCountryNode(
                    countryCode = code,
                    countryName = best.countryLong,
                    serverCount = sList.size,
                    bestPing = best.ping,
                    normalizedPos = norm,
                    representativeServer = best
                )
            }
            .sortedByDescending { it.serverCount }
    }

    var isZoomedIn by remember { mutableStateOf(false) }
    var userTappedNode by remember { mutableStateOf<MapCountryNode?>(null) }
    var viewportSize by remember { mutableStateOf(IntSize(1, 1)) }

    // Target coordinates
    val targetNorm = CountryCoordinates.getNormalizedOffset(activeCountryCode)
        ?: CountryCoordinates.getNormalizedOffset("JP")
        ?: Offset(0.8465f, 0.3417f)

    // User Origin coordinates (France default)
    val originNorm = CountryCoordinates.getNormalizedOffset("FR") ?: Offset(0.4925f, 0.2747f)

    // Smooth camera zoom and pan
    val zoomScale by animateFloatAsState(
        targetValue = if (isZoomedIn) 2.2f else 1.0f,
        animationSpec = tween(durationMillis = 600, easing = FastOutSlowInEasing),
        label = "zoomScale"
    )

    val panOffsetX by animateFloatAsState(
        targetValue = if (isZoomedIn) {
            val cx = targetNorm.x * viewportSize.width
            val maxShift = viewportSize.width * 0.5f
            ((viewportSize.width / 2f) - cx).coerceIn(-maxShift, maxShift) * 1.4f
        } else 0f,
        animationSpec = tween(durationMillis = 600, easing = FastOutSlowInEasing),
        label = "panOffsetX"
    )

    val panOffsetY by animateFloatAsState(
        targetValue = if (isZoomedIn) {
            val cy = targetNorm.y * viewportSize.height
            val maxShift = viewportSize.height * 0.5f
            ((viewportSize.height / 2f) - cy).coerceIn(-maxShift, maxShift) * 1.4f
        } else 0f,
        animationSpec = tween(durationMillis = 600, easing = FastOutSlowInEasing),
        label = "panOffsetY"
    )

    // Infinite radar pulse animations
    val infiniteTransition = rememberInfiniteTransition(label = "mapPulse")

    val pulse1 by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(1800, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "pulse1"
    )

    val pulse2 by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(1800, delayMillis = 900, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "pulse2"
    )

    val arcFlow by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(1400, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "arcFlow"
    )

    val isConnected = status == ConnectionStatus.CONNECTED
    val isConnecting = status == ConnectionStatus.CONNECTING

    val cachedArcPath = remember { Path() }

    // Outer Map Container
    Box(
        modifier = modifier
            .fillMaxWidth()
            .shadow(
                elevation = if (colors.isDark) 0.dp else 4.dp,
                shape = RoundedCornerShape(20.dp),
                ambientColor = colors.cardShadowColor,
                spotColor = colors.cardShadowColor
            )
            .clip(RoundedCornerShape(20.dp))
            .background(if (colors.isDark) Color(0xFF090C12) else Color.White)
            .border(1.dp, if (colors.isDark) Color(0xFF1E2638) else Color(0xFFCBD5E1), RoundedCornerShape(20.dp))
    ) {
        Column {
            // Viewport Box with Map & Interactive Canvas
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .aspectRatio(2000f / 1001f)
                    .clip(RoundedCornerShape(topStart = 20.dp, topEnd = 20.dp))
                    .background(if (colors.isDark) Color(0xFF070A11) else Color(0xFFE2E8F0))
                    .onSizeChanged { viewportSize = it }
            ) {
                // Layer 1: Hardware-Accelerated Vector World Map & Canvas Overlay
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .graphicsLayer {
                            scaleX = zoomScale
                            scaleY = zoomScale
                            translationX = panOffsetX
                            translationY = panOffsetY
                        }
                ) {
                    // Vector continents
                    Image(
                        painter = painterResource(
                            id = if (colors.isDark) R.drawable.world_map_dark else R.drawable.world_map_light
                        ),
                        contentDescription = "World Map",
                        contentScale = ContentScale.FillBounds,
                        modifier = Modifier.fillMaxSize()
                    )

                    // Layer 2: Glowing Points Lumineux & Flight Path Arc Canvas
                    androidx.compose.foundation.Canvas(
                        modifier = Modifier
                            .fillMaxSize()
                            .pointerInput(countryNodes) {
                                detectTapGestures { tapOffset ->
                                    val w = size.width
                                    val h = size.height
                                    var closestNode: MapCountryNode? = null
                                    var minDist = 40.dp.toPx()

                                    for (node in countryNodes) {
                                        val nx = node.normalizedPos.x * w
                                        val ny = node.normalizedPos.y * h
                                        val d = hypot(nx - tapOffset.x, ny - tapOffset.y)
                                        if (d < minDist) {
                                            minDist = d
                                            closestNode = node
                                        }
                                    }

                                    if (closestNode != null) {
                                        userTappedNode = closestNode
                                        onSelectServer(closestNode.representativeServer)
                                    } else {
                                        userTappedNode = null
                                    }
                                }
                            }
                    ) {
                        val w = size.width
                        val h = size.height

                        val originPx = Offset(originNorm.x * w, originNorm.y * h)
                        val targetPx = Offset(targetNorm.x * w, targetNorm.y * h)

                        // 1. Draw Active Connection Flight Arc
                        if (isConnected || isConnecting) {
                            val midX = (originPx.x + targetPx.x) / 2f
                            val dist = hypot(targetPx.x - originPx.x, targetPx.y - originPx.y)
                            val arcHeight = (dist * 0.32f).coerceIn(20f, 90f)
                            val ctrlY = min(originPx.y, targetPx.y) - arcHeight
                            val ctrlPx = Offset(midX, ctrlY)

                            cachedArcPath.reset()
                            cachedArcPath.moveTo(originPx.x, originPx.y)
                            cachedArcPath.quadraticTo(ctrlPx.x, ctrlPx.y, targetPx.x, targetPx.y)

                            // Glowing background arc shadow
                            drawPath(
                                path = cachedArcPath,
                                color = if (colors.isDark) Color(0xFF10B981).copy(alpha = 0.25f) else Color(0xFF059669).copy(alpha = 0.2f),
                                style = Stroke(width = 6.dp.toPx(), cap = StrokeCap.Round)
                            )

                            // Main dashed gradient arc
                            val arcBrush = Brush.linearGradient(
                                colors = listOf(
                                    Color(0xFF3B82F6),
                                    if (isConnected) Color(0xFF10B981) else Color(0xFFF59E0B)
                                ),
                                start = originPx,
                                end = targetPx
                            )

                            drawPath(
                                path = cachedArcPath,
                                brush = arcBrush,
                                style = Stroke(
                                    width = 2.5.dp.toPx(),
                                    cap = StrokeCap.Round,
                                    pathEffect = PathEffect.dashPathEffect(
                                        intervals = ArcDashIntervals,
                                        phase = -arcFlow * 26f
                                    )
                                )
                            )

                            // Traveling Glowing Comet Particle along quadratic bezier: B(t) = (1-t)^2 P0 + 2(1-t)t P1 + t^2 P2
                            val t = arcFlow
                            val cometX = (1f - t) * (1f - t) * originPx.x + 2f * (1f - t) * t * ctrlPx.x + t * t * targetPx.x
                            val cometY = (1f - t) * (1f - t) * originPx.y + 2f * (1f - t) * t * ctrlPx.y + t * t * targetPx.y
                            val cometPos = Offset(cometX, cometY)

                            // Comet halo
                            drawCircle(
                                color = Color(0xFF38BDF8).copy(alpha = 0.5f),
                                radius = 6.5.dp.toPx(),
                                center = cometPos
                            )
                            // Comet center
                            drawCircle(
                                color = Color.White,
                                radius = 3.dp.toPx(),
                                center = cometPos
                            )
                        }

                        // 2. Draw User Local Node (France / Client Location)
                        drawCircle(
                            color = Color(0xFF3B82F6).copy(alpha = 0.35f),
                            radius = 7.dp.toPx(),
                            center = originPx
                        )
                        drawCircle(
                            color = Color(0xFF3B82F6),
                            radius = 3.5.dp.toPx(),
                            center = originPx
                        )
                        drawCircle(
                            color = Color.White,
                            radius = 1.8.dp.toPx(),
                            center = originPx
                        )

                        // 3. Draw All Server Points Lumineux
                        for (node in countryNodes) {
                            val pos = Offset(node.normalizedPos.x * w, node.normalizedPos.y * h)
                            val isTarget = node.countryCode.equals(activeCountryCode, ignoreCase = true)

                            if (isTarget) {
                                // Pulsing Radar Beacon Waves
                                val r1 = (5.dp.toPx() + (22.dp.toPx() * pulse1))
                                val a1 = (1f - pulse1) * 0.75f
                                val r2 = (5.dp.toPx() + (22.dp.toPx() * pulse2))
                                val a2 = (1f - pulse2) * 0.75f

                                val beaconColor = when {
                                    isConnected -> Color(0xFF10B981)
                                    isConnecting -> Color(0xFFF59E0B)
                                    else -> if (colors.isDark) Color(0xFF38BDF8) else Color(0xFF0284C7)
                                }

                                drawCircle(
                                    color = beaconColor.copy(alpha = a1),
                                    radius = r1,
                                    center = pos,
                                    style = Stroke(width = 1.8.dp.toPx())
                                )
                                drawCircle(
                                    color = beaconColor.copy(alpha = a2),
                                    radius = r2,
                                    center = pos,
                                    style = Stroke(width = 1.8.dp.toPx())
                                )

                                // Solid high-contrast core
                                drawCircle(
                                    color = beaconColor.copy(alpha = 0.4f),
                                    radius = 8.dp.toPx(),
                                    center = pos
                                )
                                drawCircle(
                                    color = if (colors.isDark) Color(0xFF07090E) else Color.White,
                                    radius = 5.dp.toPx(),
                                    center = pos
                                )
                                drawCircle(
                                    color = beaconColor,
                                    radius = 3.8.dp.toPx(),
                                    center = pos
                                )
                            } else {
                                // Subtle Glowing Point Lumineux for inactive nodes
                                val nodeColor = when {
                                    node.bestPing < 50 -> Color(0xFF10B981)
                                    node.bestPing < 160 -> Color(0xFF06B6D4)
                                    else -> Color(0xFFF59E0B)
                                }

                                drawCircle(
                                    color = nodeColor.copy(alpha = 0.35f),
                                    radius = 4.5.dp.toPx(),
                                    center = pos
                                )
                                drawCircle(
                                    color = if (colors.isDark) nodeColor else nodeColor.copy(alpha = 0.9f),
                                    radius = 2.4.dp.toPx(),
                                    center = pos
                                )
                            }
                        }
                    }
                }

                // Top Header Overlay: Live Status Pill & Recenter Camera Button
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 12.dp, vertical = 10.dp),
                    horizontalArrangement = Arrangement.SpaceBetween,
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    // Status Badge
                    Box(
                        modifier = Modifier
                            .clip(RoundedCornerShape(20.dp))
                            .background(
                                if (colors.isDark) Color(0xFF0F1522).copy(alpha = 0.88f)
                                else Color.White.copy(alpha = 0.94f)
                            )
                            .border(
                                1.dp,
                                if (colors.isDark) Color(0xFF263248) else Color(0xFFCBD5E1),
                                RoundedCornerShape(20.dp)
                            )
                            .padding(horizontal = 10.dp, vertical = 5.dp)
                    ) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            val dotColor = when {
                                isConnected -> Color(0xFF10B981)
                                isConnecting -> Color(0xFFF59E0B)
                                else -> Color(0xFF38BDF8)
                            }
                            Box(
                                modifier = Modifier
                                    .size(7.dp)
                                    .clip(CircleShape)
                                    .background(dotColor)
                            )
                            Spacer(modifier = Modifier.width(6.dp))

                            val flagEmoji = CountryCoordinates.countryCodeToEmoji(activeCountryCode)
                            val statusLabel = when {
                                isConnected -> "$flagEmoji ${activeServer?.countryLong ?: activeCountryCode} • ${activeServer?.ping ?: 0}ms"
                                isConnecting -> strings.statusConnecting
                                else -> "$flagEmoji ${activeServer?.countryLong ?: activeCountryCode} (${countryNodes.size} nodes)"
                            }

                            Text(
                                text = statusLabel,
                                color = colors.textPrimary,
                                fontSize = 11.sp,
                                fontWeight = FontWeight.SemiBold
                            )
                        }
                    }

                    // Recenter / Zoom Toggle Button
                    IconButton(
                        onClick = { isZoomedIn = !isZoomedIn },
                        modifier = Modifier
                            .size(32.dp)
                            .clip(CircleShape)
                            .background(
                                if (colors.isDark) Color(0xFF0F1522).copy(alpha = 0.88f)
                                else Color.White.copy(alpha = 0.94f)
                            )
                            .border(
                                1.dp,
                                if (colors.isDark) Color(0xFF263248) else Color(0xFFCBD5E1),
                                CircleShape
                            )
                    ) {
                        Icon(
                            imageVector = Icons.Default.CenterFocusStrong,
                            contentDescription = "Recenter Map",
                            tint = if (isZoomedIn) Color(0xFF10B981) else colors.textMuted,
                            modifier = Modifier.size(17.dp)
                        )
                    }
                }

                // Node Tap Callout Pill
                if (userTappedNode != null) {
                    val tapped = userTappedNode!!
                    Box(
                        modifier = Modifier
                            .align(Alignment.BottomCenter)
                            .padding(bottom = 8.dp)
                            .clip(RoundedCornerShape(12.dp))
                            .background(if (colors.isDark) Color(0xFF131A29) else Color.White)
                            .border(1.dp, Color(0xFF10B981), RoundedCornerShape(12.dp))
                            .padding(horizontal = 12.dp, vertical = 6.dp)
                    ) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text(
                                text = CountryCoordinates.countryCodeToEmoji(tapped.countryCode),
                                fontSize = 14.sp
                            )
                            Spacer(modifier = Modifier.width(6.dp))
                            Text(
                                text = "${tapped.countryName} • ${tapped.serverCount} servers • ${tapped.bestPing}ms",
                                color = colors.textPrimary,
                                fontSize = 11.sp,
                                fontWeight = FontWeight.Medium
                            )
                        }
                    }
                }
            }

            // Bottom Horizontal Quick Switcher Carousel
            if (countryNodes.isNotEmpty()) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .background(if (colors.isDark) Color(0xFF0B0E17) else Color(0xFFF8FAFC))
                        .horizontalScroll(rememberScrollState())
                        .padding(horizontal = 10.dp, vertical = 8.dp),
                    horizontalArrangement = Arrangement.spacedBy(6.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Icon(
                        imageVector = Icons.Default.Public,
                        contentDescription = null,
                        tint = colors.textMuted,
                        modifier = Modifier
                            .size(16.dp)
                            .padding(start = 2.dp)
                    )

                    countryNodes.take(12).forEach { node ->
                        val isSelected = node.countryCode.equals(activeCountryCode, ignoreCase = true)
                        val chipBg = when {
                            isSelected -> if (colors.isDark) Color(0xFF143026) else Color(0xFFD1FAE5)
                            else -> if (colors.isDark) Color(0xFF141923) else Color.White
                        }
                        val chipBorder = when {
                            isSelected -> Color(0xFF10B981)
                            else -> if (colors.isDark) Color(0xFF222B3D) else Color(0xFFE2E8F0)
                        }

                        Box(
                            modifier = Modifier
                                .clip(RoundedCornerShape(14.dp))
                                .background(chipBg)
                                .border(1.dp, chipBorder, RoundedCornerShape(14.dp))
                                .clickable {
                                    onSelectServer(node.representativeServer)
                                }
                                .padding(horizontal = 9.dp, vertical = 4.dp)
                        ) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Text(
                                    text = CountryCoordinates.countryCodeToEmoji(node.countryCode),
                                    fontSize = 11.sp
                                )
                                Spacer(modifier = Modifier.width(5.dp))
                                Text(
                                    text = node.countryCode,
                                    color = if (isSelected) Color(0xFF10B981) else colors.textPrimary,
                                    fontSize = 11.sp,
                                    fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Medium
                                )
                                Spacer(modifier = Modifier.width(4.dp))
                                Text(
                                    text = "(${node.serverCount})",
                                    color = colors.textMuted,
                                    fontSize = 9.sp
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}
