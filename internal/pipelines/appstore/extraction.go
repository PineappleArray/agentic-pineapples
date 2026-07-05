package appstore

import (
	"context"

	"github.com/PineappleArray/agentic-pineapples/internal/types"
)

type AppleExtractor struct {
	appId   string
	appName string
}

func (ae *AppleExtractor) Extractor(ctx context.Context, ev types.Event) (types.Extraction, error) {

}
