package main

import (
	"os"

	"github.com/davegallant/vpngate/cmd"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	level := zerolog.InfoLevel
	if os.Getenv("VPNGATE_DEBUG") != "" {
		level = zerolog.DebugLevel
	}
	zerolog.SetGlobalLevel(level)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	cmd.Execute()
}
