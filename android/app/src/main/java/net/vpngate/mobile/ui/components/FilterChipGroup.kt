package net.vpngate.mobile.ui.components

import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FilterChipDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import net.vpngate.mobile.ui.theme.AppTheme

@Composable
fun FilterChipGroup(
    countries: List<String>,
    selectedCountry: String?,
    onSelectCountry: (String?) -> Unit,
    modifier: Modifier = Modifier
) {
    val colors = AppTheme.colors
    val scrollState = rememberScrollState()

    Row(
        modifier = modifier
            .fillMaxWidth()
            .horizontalScroll(scrollState)
            .padding(horizontal = 4.dp)
    ) {
        FilterChip(
            selected = selectedCountry == null,
            onClick = { onSelectCountry(null) },
            label = { Text("All", fontSize = 12.sp) },
            colors = FilterChipDefaults.filterChipColors(
                containerColor = colors.surface,
                labelColor = colors.textSecondary,
                selectedContainerColor = colors.accentPrimary,
                selectedLabelColor = if (colors.isDark) Color(0xFF09090B) else Color.White
            ),
            border = FilterChipDefaults.filterChipBorder(
                borderColor = colors.border,
                selectedBorderColor = colors.accentPrimary,
                enabled = true,
                selected = selectedCountry == null
            )
        )

        countries.forEach { code ->
            Spacer(modifier = Modifier.width(6.dp))
            val isSelected = selectedCountry == code
            FilterChip(
                selected = isSelected,
                onClick = { onSelectCountry(if (isSelected) null else code) },
                label = { Text(code, fontSize = 12.sp) },
                colors = FilterChipDefaults.filterChipColors(
                    containerColor = colors.surface,
                    labelColor = colors.textPrimary,
                    selectedContainerColor = colors.accentPrimary,
                    selectedLabelColor = if (colors.isDark) Color(0xFF09090B) else Color.White
                ),
                border = FilterChipDefaults.filterChipBorder(
                    borderColor = colors.border,
                    selectedBorderColor = colors.accentPrimary,
                    enabled = true,
                    selected = isSelected
                )
            )
        }
    }
}
