package net.vpngate.mobile.util

import net.vpngate.mobile.data.model.VpnServer

object CsvParser {

    fun parseVpnList(csvContent: String): List<VpnServer> {
        val lines = csvContent.lineSequence()
        val servers = mutableListOf<VpnServer>()

        for (rawLine in lines) {
            val line = rawLine.trim()
            if (line.isEmpty() || line.startsWith("*") || line.startsWith("#")) {
                continue
            }

            val fields = parseCsvLine(line)
            if (fields.size < 15) {
                continue
            }

            try {
                val server = VpnServer(
                    hostName = fields[0],
                    ip = fields[1],
                    score = fields[2].toLongOrNull() ?: 0L,
                    ping = fields[3].toLongOrNull() ?: 999L,
                    speed = fields[4].toLongOrNull() ?: 0L,
                    countryLong = fields[5],
                    countryShort = fields[6],
                    numVpnSessions = fields[7].toLongOrNull() ?: 0L,
                    uptime = fields[8].toLongOrNull() ?: 0L,
                    totalUsers = fields[9].toLongOrNull() ?: 0L,
                    totalTraffic = fields[10].toLongOrNull() ?: 0L,
                    logType = fields[11],
                    operator = fields[12],
                    message = fields[13],
                    openVpnConfigDataBase64 = fields[14]
                )
                servers.add(server)
            } catch (_: Exception) {
                // Ignore malformed individual rows
            }
        }

        return servers
    }

    private fun parseCsvLine(line: String): List<String> {
        val result = mutableListOf<String>()
        val sb = StringBuilder()
        var inQuotes = false

        for (i in line.indices) {
            val c = line[i]
            when {
                c == '\"' -> {
                    inQuotes = !inQuotes
                }
                c == ',' && !inQuotes -> {
                    result.add(sb.toString().trim())
                    sb.clear()
                }
                else -> {
                    sb.append(c)
                }
            }
        }
        result.add(sb.toString().trim())
        return result
    }
}
