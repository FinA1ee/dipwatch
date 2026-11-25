package jobs

import (
	"context"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

// Simulator queues simulated price drops for the admin API.
type Simulator struct {
	Queue interface {
		Insert(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) (*rivertype.JobInsertResult, error)
	}
}

func (s Simulator) SimulateDrop(ctx context.Context, productID int64, percent int) error {
	_, err := s.Queue.Insert(ctx, SimulateDropArgs{ProductID: productID, Percent: percent}, nil)
	return err
}
