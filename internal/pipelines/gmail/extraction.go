package gmail

import (
	"context"

	"github.com/PineappleArray/agentic-pineapples/internal/types"
)

type GmailExtractor struct {
	gmailEmail string
}

func (ae *GmailExtractor) Extractor(ctx context.Context, ev types.Event) (types.Extraction, error) {

}
