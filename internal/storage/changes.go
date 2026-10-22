package storage

import "context"

// ChangeFeed reports which workspaces committed writes change, whichever
// process made them.
type ChangeFeed interface {
	// Listen calls changed with the workspace of every change committed after
	// it calls ready, until ctx ends or the feed fails.
	Listen(ctx context.Context, ready func(), changed func(workspaceID string)) error
}
