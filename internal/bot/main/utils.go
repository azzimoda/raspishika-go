package mainbot

import (
	"context"

	"github.com/rs/zerolog/log"
)

// addContextHandlerError adds an error to the handler error context.
func addContextHandlerError(ctx context.Context, err error) {
	handlerErrs, ok := ctx.Value(errorContextKey).(*[]error)
	if ok {
		if err != nil {
			*handlerErrs = append(*handlerErrs, err)
		}
	} else {
		log.Warn().Err(err).Msg("Error context not found")
	}
}

func shortenText(text string, maxLength int) string {
	if len(text) > maxLength {
		return text[:maxLength-2] + "…"
	}
	return text
}
