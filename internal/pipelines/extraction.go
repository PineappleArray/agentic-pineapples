package pipelines

import (
	"context"

	"github.com/PineappleArray/agentic-pineapples/internal/types"
)

type Extractor interface {
	Extract(ctx context.Context, ev types.Event) (types.Extraction, error)
}
