package influxdb

import (
	"context"
	"fmt"

	"github.com/influxdata/influxdb/v2/kit/platform"

	"github.com/influxdata/flux"
	"github.com/influxdata/flux/codes"
	"github.com/influxdata/flux/plan"
	"github.com/influxdata/flux/values"
	"github.com/influxdata/influxdb/v2/storage/reads/datatypes"
)

const (
	ReadRangePhysKind           = "ReadRangePhysKind"
	ReadGroupPhysKind           = "ReadGroupPhysKind"
	ReadWindowAggregatePhysKind = "ReadWindowAggregatePhysKind"
	ReadTagKeysPhysKind         = "ReadTagKeysPhysKind"
	ReadTagValuesPhysKind       = "ReadTagValuesPhysKind"
)

type ReadGroupPhysSpec struct {
	plan.DefaultCost
	ReadRangePhysSpec

	GroupMode flux.GroupMode
	GroupKeys []string

	AggregateMethod string
}

func (s *ReadGroupPhysSpec) PlanDetails() string {
	return fmt.Sprintf("GroupMode: %v, GroupKeys: %v, AggregateMethod: \"%s\"", s.GroupMode, s.GroupKeys, s.AggregateMethod)
}

func (s *ReadGroupPhysSpec) Kind() plan.ProcedureKind {
	return ReadGroupPhysKind
}

func (s *ReadGroupPhysSpec) Copy() plan.ProcedureSpec {
	ns := new(ReadGroupPhysSpec)
	ns.ReadRangePhysSpec = *s.ReadRangePhysSpec.Copy().(*ReadRangePhysSpec)

	ns.GroupMode = s.GroupMode
	ns.GroupKeys = s.GroupKeys

	ns.AggregateMethod = s.AggregateMethod
	return ns
}

type ReadRangePhysSpec struct {
	plan.DefaultCost

	Bucket   string
	BucketID string

	// Filter is the filter to use when calling into
	// storage. It must be possible to push down this
	// filter.
	Filter *datatypes.Predicate

	Bounds flux.Bounds
}

func (s *ReadRangePhysSpec) Kind() plan.ProcedureKind {
	return ReadRangePhysKind
}
func (s *ReadRangePhysSpec) Copy() plan.ProcedureSpec {
	ns := *s
	return &ns
}

func (s *ReadRangePhysSpec) LookupBucketID(ctx context.Context, orgID platform.ID, buckets BucketLookup) (platform.ID, error) {
	// Determine bucketID
	switch {
	case s.Bucket != "":
		b, ok := buckets.Lookup(ctx, orgID, s.Bucket)
		if !ok {
			return 0, &flux.Error{
				Code: codes.NotFound,
				Msg:  fmt.Sprintf("could not find bucket %q", s.Bucket),
			}
		}
		return b, nil
	case len(s.BucketID) != 0:
		var b platform.ID
		if err := b.DecodeFromString(s.BucketID); err != nil {
			return 0, &flux.Error{
				Code: codes.Invalid,
				Msg:  "invalid bucket id",
				Err:  err,
			}
		}
		return b, nil
	default:
		return 0, &flux.Error{
			Code: codes.Invalid,
			Msg:  "no bucket name or id have been specified",
		}
	}
}

// TimeBounds implements plan.BoundsAwareProcedureSpec.
func (s *ReadRangePhysSpec) TimeBounds(predecessorBounds *plan.Bounds) *plan.Bounds {
	return &plan.Bounds{
		Start: values.ConvertTime(s.Bounds.Start.Time(s.Bounds.Now)),
		Stop:  values.ConvertTime(s.Bounds.Stop.Time(s.Bounds.Now)),
	}
}

type ReadWindowAggregatePhysSpec struct {
	plan.DefaultCost
	ReadRangePhysSpec

	WindowEvery flux.Duration
	Offset      flux.Duration
	Aggregates  []plan.ProcedureKind
	CreateEmpty bool
	TimeColumn  string
}

func (s *ReadWindowAggregatePhysSpec) PlanDetails() string {
	return fmt.Sprintf("every = %v, aggregates = %v, createEmpty = %v, timeColumn = \"%s\"", s.WindowEvery, s.Aggregates, s.CreateEmpty, s.TimeColumn)
}

func (s *ReadWindowAggregatePhysSpec) Kind() plan.ProcedureKind {
	return ReadWindowAggregatePhysKind
}

func (s *ReadWindowAggregatePhysSpec) Copy() plan.ProcedureSpec {
	ns := new(ReadWindowAggregatePhysSpec)

	ns.ReadRangePhysSpec = *s.ReadRangePhysSpec.Copy().(*ReadRangePhysSpec)
	ns.WindowEvery = s.WindowEvery
	ns.Offset = s.Offset
	ns.Aggregates = s.Aggregates
	ns.CreateEmpty = s.CreateEmpty
	ns.TimeColumn = s.TimeColumn

	return ns
}

type ReadTagKeysPhysSpec struct {
	ReadRangePhysSpec
}

func (s *ReadTagKeysPhysSpec) Kind() plan.ProcedureKind {
	return ReadTagKeysPhysKind
}

func (s *ReadTagKeysPhysSpec) Copy() plan.ProcedureSpec {
	ns := new(ReadTagKeysPhysSpec)
	ns.ReadRangePhysSpec = *s.ReadRangePhysSpec.Copy().(*ReadRangePhysSpec)
	return ns
}

type ReadTagValuesPhysSpec struct {
	ReadRangePhysSpec
	TagKey string
}

func (s *ReadTagValuesPhysSpec) Kind() plan.ProcedureKind {
	return ReadTagValuesPhysKind
}

func (s *ReadTagValuesPhysSpec) Copy() plan.ProcedureSpec {
	ns := new(ReadTagValuesPhysSpec)
	ns.ReadRangePhysSpec = *s.ReadRangePhysSpec.Copy().(*ReadRangePhysSpec)
	ns.TagKey = s.TagKey
	return ns
}
