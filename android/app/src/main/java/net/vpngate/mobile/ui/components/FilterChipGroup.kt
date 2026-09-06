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
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import net.vpngate.mobile.ui.theme.Emerald500
import net.vpngate.mobile.ui.theme.Zinc100
import net.vpngate.mobile.ui.theme.Zinc400
import net.vpngate.mobile.ui.theme.Zinc800
import net.vpngate.mobile.ui.theme.Zinc900
import net.vpngate.mobile.ui.theme.Zinc950

@Composable
fun FilterChipGroup(
    countries: List<String>,
    selectedCountry: String?,
    onSelectCountry: (String?) -> Unit,
    modifier: Modifier = Modifier
) {
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
                containerColor = Zinc900,
                labelColor = Zinc400,
                selectedContainerColor = Emerald500,
                selectedLabelColor = Zinc950
            ),
            border = FilterChipDefaults.filterChipBorder(
                borderColor = Zinc800,
                selectedBorderColor = Emerald500,
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
                    containerColor = Zinc900,
                    labelColor = Zinc100,
                    selectedContainerColor = Emerald500,
                    selectedLabelColor = Zinc950
                ),
                border = FilterChipDefaults.filterChipBorder(
                    borderColor = Zinc800,
                    selectedBorderColor = Emerald500,
                    enabled = true,
                    selected = isSelected
                )
            )
        }
    }
}
