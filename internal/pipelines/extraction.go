package pipelines

import (
	"context"

	"../types"
)

type Extractor interface {
	Extract(ctx context.Context, ev types.Event) (types.Extraction, error)
}
