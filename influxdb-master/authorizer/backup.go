package authorizer

import (
	"context"
	"io"
	"time"

	"github.com/influxdata/influxdb/v2"
	"github.com/influxdata/influxdb/v2/kit/tracing"
)

var _ influxdb.BackupService = (*BackupService)(nil)

// BackupService wraps a influxdb.BackupService and authorizes actions
// against it appropriately.
type BackupService struct {
	s influxdb.BackupService
}

// NewBackupService constructs an instance of an authorizing backup service.
func NewBackupService(s influxdb.BackupService) *BackupService {
	return &BackupService{
		s: s,
	}
}

func (b BackupService) BackupKVStore(ctx context.Context, w io.Writer) error {
	span, ctx := tracing.StartSpanFromContext(ctx)
	defer span.Finish()

	if err := IsAllowedAll(ctx, influxdb.OperPermissions()); err != nil {
		return err
	}
	return b.s.BackupKVStore(ctx, w)
}

func (b BackupService) BackupShard(ctx context.Context, w io.Writer, shardID uint64, since time.Time) error {
	span, ctx := tracing.StartSpanFromContext(ctx)
	defer span.Finish()

	if err := IsAllowedAll(ctx, influxdb.OperPermissions()); err != nil {
		return err
	}
	return b.s.BackupShard(ctx, w, shardID, since)
}
