package net.vpngate.mobile

import net.vpngate.mobile.ui.components.map.CountryCoordinates
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class CountryCoordinatesTest {

    @Test
    fun testCountryCoordinatesLookup() {
        val jp = CountryCoordinates.getNormalizedOffset("JP")
        assertNotNull(jp)
        assertTrue("JP x should be in eastern hemisphere", jp!!.x > 0.7f)
        assertTrue("JP y should be in northern hemisphere", jp.y in 0.2f..0.5f)

        val fr = CountryCoordinates.getNormalizedOffset("FR")
        assertNotNull(fr)
        assertTrue("FR x should be central Europe", fr!!.x in 0.45f..0.55f)

        val us = CountryCoordinates.getNormalizedOffset("US")
        assertNotNull(us)
        assertTrue("US x should be western hemisphere", us!!.x in 0.15f..0.35f)

        assertNull(CountryCoordinates.getNormalizedOffset(null))
        assertNull(CountryCoordinates.getNormalizedOffset(""))
        assertNull(CountryCoordinates.getNormalizedOffset("INVALID_COUNTRY"))
    }

    @Test
    fun testCountryEmoji() {
        assertEquals("🇯🇵", CountryCoordinates.countryCodeToEmoji("JP"))
        assertEquals("🇫🇷", CountryCoordinates.countryCodeToEmoji("FR"))
        assertEquals("🇺🇸", CountryCoordinates.countryCodeToEmoji("US"))
        assertEquals("🇩🇪", CountryCoordinates.countryCodeToEmoji("DE"))
        assertEquals("🌐", CountryCoordinates.countryCodeToEmoji(null))
        assertEquals("🌐", CountryCoordinates.countryCodeToEmoji(""))
        assertEquals("🌐", CountryCoordinates.countryCodeToEmoji("XYZ"))
    }
}
