// Package logging centralizes zerolog configuration so both entrypoints
// (api, worker) produce consistent structured logs.
package logging

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Configure sets up the global logger. In development it uses a human-friendly
// console writer at debug level; otherwise it emits JSON at info level.
func Configure(development bool) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	if development {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		return
	}
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}
