package net.vpngate.mobile

import net.vpngate.mobile.util.CsvParser
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Test

class CsvParserTest {

    private val sampleCsv = """
        *vpn_servers
        #HostName,IP,Score,Ping,Speed,CountryLong,CountryShort,NumVpnSessions,Uptime,TotalUsers,TotalTraffic,LogType,Operator,Message,OpenVPN_ConfigData_Base64
        vpn123.opengw.net,192.168.1.100,543210,12,104857600,Japan,JP,15,360000,1200,99999999,2weeks,operator1,Hello World,ZHVtbXlfY29uZmln
        vpn456.opengw.net,10.0.0.50,123450,45,52428800,United States,US,5,180000,500,44444444,2weeks,operator2,Test Server,ZHVtbXlfY29uZmlnMg==
        *
    """.trimIndent()

    @Test
    fun testParseVpnList() {
        val servers = CsvParser.parseVpnList(sampleCsv)
        assertEquals(2, servers.size)

        val first = servers[0]
        assertEquals("vpn123.opengw.net", first.hostName)
        assertEquals("192.168.1.100", first.ip)
        assertEquals(543210L, first.score)
        assertEquals(12L, first.ping)
        assertEquals(104857600L, first.speed)
        assertEquals("Japan", first.countryLong)
        assertEquals("JP", first.countryShort)
        assertEquals("🇯🇵", first.flagEmoji)
        assertEquals("ZHVtbXlfY29uZmln", first.openVpnConfigDataBase64)

        val second = servers[1]
        assertEquals("United States", second.countryLong)
        assertEquals("US", second.countryShort)
        assertEquals("🇺🇸", second.flagEmoji)
    }
}
