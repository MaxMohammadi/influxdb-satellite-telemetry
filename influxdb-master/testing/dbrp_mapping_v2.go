package testing

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/influxdata/influxdb/v2/kit/platform"
	errors2 "github.com/influxdata/influxdb/v2/kit/platform/errors"

	"github.com/google/go-cmp/cmp"
	"github.com/influxdata/influxdb/v2"
	"github.com/influxdata/influxdb/v2/dbrp"
	"github.com/influxdata/influxdb/v2/mock"
	"github.com/pkg/errors"
)

var DBRPMappingCmpOptionsV2 = cmp.Options{
	cmp.Comparer(func(x, y []byte) bool {
		return bytes.Equal(x, y)
	}),
	cmp.Transformer("Sort", func(in []*influxdb.DBRPMappingV2) []*influxdb.DBRPMappingV2 {
		out := make([]*influxdb.DBRPMappingV2, len(in))
		copy(out, in) // Copy input slice to avoid mutating it
		sort.Slice(out, func(i, j int) bool {
			return out[i].ID < out[j].ID
		})
		return out
	}),
}

type DBRPMappingFieldsV2 struct {
	BucketSvc      influxdb.BucketService
	DBRPMappingsV2 []*influxdb.DBRPMappingV2
}

// Populate creates all entities in DBRPMappingFieldsV2.
func (f DBRPMappingFieldsV2) Populate(ctx context.Context, s influxdb.DBRPMappingServiceV2) error {
	for _, m := range f.DBRPMappingsV2 {
		if err := s.Create(ctx, m); err != nil {
			return errors.Wrap(err, "failed to populate dbrp mappings")
		}
	}
	return nil
}

// DBRPMappingServiceV2 tests all the service functions.
func DBRPMappingServiceV2(
	init func(DBRPMappingFieldsV2, *testing.T) (influxdb.DBRPMappingServiceV2, func()),
	t *testing.T,
) {
	tests := []struct {
		name string
		fn   func(init func(DBRPMappingFieldsV2, *testing.T) (influxdb.DBRPMappingServiceV2, func()),
			t *testing.T)
	}{
		{
			name: "create",
			fn:   CreateDBRPMappingV2,
		},
		{
			name: "find by ID",
			fn:   FindDBRPMappingByIDV2,
		},
		{
			name: "find",
			fn:   FindManyDBRPMappingsV2,
		},
		{
			name: "update",
			fn:   UpdateDBRPMappingV2,
		},
		{
			name: "delete",
			fn:   DeleteDBRPMappingV2,
		},
		{
			name: "miscellaneous",
			fn:   MiscDBRPMappingV2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt
			t.Parallel()
			tt.fn(init, t)
		})
	}
}

// CleanupDBRPMappingsV2 finds and removes all dbrp mappings.
func CleanupDBRPMappingsV2(ctx context.Context, s influxdb.DBRPMappingServiceV2) error {
	mappings, _, err := s.FindMany(ctx, influxdb.DBRPMappingFilterV2{})
	if err != nil {
		return errors.Wrap(err, "failed to retrieve all dbrp mappings")
	}

	for _, m := range mappings {
		if err := s.Delete(ctx, m.OrganizationID, m.ID); err != nil {
			return errors.Wrapf(err, "failed to remove dbrp mapping %v", m.ID)
		}
	}
	return nil
}

func CreateDBRPMappingV2(
	init func(DBRPMappingFieldsV2, *testing.T) (influxdb.DBRPMappingServiceV2, func()),
	t *testing.T,
) {
	type args struct {
		dbrpMapping *influxdb.DBRPMappingV2
	}
	type wants struct {
		err          error
		dbrpMappings []*influxdb.DBRPMappingV2
	}

	tests := []struct {
		name   string
		fields DBRPMappingFieldsV2
		args   args
		wants  wants
	}{
		{
			name: "basic create dbrp",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         false,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					// If there is only one mapping for a database, that is the default one.
					Default:        true,
					OrganizationID: MustIDBase16(dbrpOrg1ID),
					BucketID:       MustIDBase16(dbrpBucket1ID),
				}},
			},
		},
		{
			name: "create mapping for same db does not change default",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
				},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					ID:              200,
					Database:        "database1",
					RetentionPolicy: "retention_policy2",
					Default:         false,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              200,
						Database:        "database1",
						RetentionPolicy: "retention_policy2",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
				},
			},
		},
		{
			name: "create mapping for same db changes default",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
				},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					ID:              200,
					Database:        "database1",
					RetentionPolicy: "retention_policy2",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              200,
						Database:        "database1",
						RetentionPolicy: "retention_policy2",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
				},
			},
		},
		{
			name: "error on create existing dbrp with same ID",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{{
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         false,
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					// NOTE(affo): in the "same ID" concept, orgID must match too!
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					ID:              100,
					Database:        "database2",
					RetentionPolicy: "retention_policy2",
					Default:         false,
					BucketID:        MustIDBase16(dbrpBucket2ID),
				},
			},
			wants: wants{
				err: dbrp.ErrDBRPAlreadyExists("dbrp already exist for this particular ID. If you are trying an update use the right function .Update"),
				dbrpMappings: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
		},
		{
			name: "error on create dbrp with same orgID, db and rp",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{{
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         false,
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					ID:              200,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         false,
					BucketID:        MustIDBase16(dbrpBucket2ID),
				},
			},
			wants: wants{
				err: dbrp.ErrDBRPAlreadyExists("another DBRP mapping with same orgID, db, and rp exists"),
				dbrpMappings: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
		},
		{
			name: "error bucket does not exist",
			fields: DBRPMappingFieldsV2{
				BucketSvc: &mock.BucketService{
					FindBucketByIDFn: func(ctx context.Context, id platform.ID) (*influxdb.Bucket, error) {
						if id == MustIDBase16(dbrpBucket2ID) {
							return nil, &errors2.Error{
								Code: errors2.ENotFound,
								Msg:  "bucket not found",
							}
						}
						return nil, nil
					},
				},
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         false,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					ID:              200,
					Database:        "database1",
					RetentionPolicy: "retention_policy2",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket2ID),
				},
			},
			wants: wants{
				err: &errors2.Error{
					Code: errors2.ENotFound,
					Msg:  "bucket not found",
				},
				dbrpMappings: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, done := init(tt.fields, t)
			defer done()
			ctx := context.Background()
			err := s.Create(ctx, tt.args.dbrpMapping)
			if (err != nil) != (tt.wants.err != nil) {
				t.Errorf("expected error '%v' got '%v'", tt.wants.err, err)
			}

			if err != nil && tt.wants.err != nil {
				if err.Error() != tt.wants.err.Error() {
					t.Errorf("expected error messages to match '%v' got '%v'", tt.wants.err, err.Error())
				}
			}

			dbrpMappings, n, err := s.FindMany(ctx, influxdb.DBRPMappingFilterV2{})
			if err != nil {
				t.Fatalf("failed to retrieve dbrps: %v", err)
			}
			if n != len(tt.wants.dbrpMappings) {
				t.Errorf("want dbrpMappings count of %d, got %d", len(tt.wants.dbrpMappings), n)
			}
			if diff := cmp.Diff(tt.wants.dbrpMappings, dbrpMappings, DBRPMappingCmpOptionsV2...); diff != "" {
				t.Errorf("dbrpMappings are different -want/+got\ndiff %s", diff)
			}
		})
	}
}

func FindManyDBRPMappingsV2(
	init func(DBRPMappingFieldsV2, *testing.T) (influxdb.DBRPMappingServiceV2, func()),
	t *testing.T,
) {
	type args struct {
		filter influxdb.DBRPMappingFilterV2
	}

	type wants struct {
		dbrpMappings []*influxdb.DBRPMappingV2
		err          error
	}
	tests := []struct {
		name   string
		fields DBRPMappingFieldsV2
		args   args
		wants  wants
	}{
		{
			name: "find all dbrps",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              200,
						Database:        "database2",
						RetentionPolicy: "retention_policy2",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucket2ID),
					},
				},
			},
			args: args{
				filter: influxdb.DBRPMappingFilterV2{},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              200,
						Database:        "database2",
						RetentionPolicy: "retention_policy2",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucket2ID),
					},
				},
			},
		},
		{
			name: "find by ID",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              MustIDBase16("1111111111111111"),
						Database:        "database1",
						RetentionPolicy: "retention_policyA",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						ID:              MustIDBase16("2222222222222222"),
						Database:        "database2",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
			args: args{
				filter: influxdb.DBRPMappingFilterV2{
					ID: MustIDBase16Ptr("1111111111111111"),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              MustIDBase16("1111111111111111"),
						Database:        "database1",
						RetentionPolicy: "retention_policyA",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
				},
			},
		},
		{
			name: "find by bucket ID",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policyA",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						ID:              200,
						Database:        "database2",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
			args: args{
				filter: influxdb.DBRPMappingFilterV2{
					BucketID: MustIDBase16Ptr(dbrpBucketBID),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              200,
						Database:        "database2",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
		},
		{
			name: "find by orgID",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database",
						RetentionPolicy: "retention_policyA",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						ID:              200,
						Database:        "database",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
					{
						ID:              300,
						Database:        "database1",
						RetentionPolicy: "retention_policyA",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
					{
						ID:              400,
						Database:        "database1",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
					{
						ID:              500,
						Database:        "database2",
						RetentionPolicy: "retention_policyA",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
			args: args{
				filter: influxdb.DBRPMappingFilterV2{
					OrgID: MustIDBase16Ptr(dbrpOrg3ID),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              300,
						Database:        "database1",
						RetentionPolicy: "retention_policyA",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
					{
						ID:              400,
						Database:        "database1",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
					{
						ID:              500,
						Database:        "database2",
						RetentionPolicy: "retention_policyA",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
		},
		{
			name: "find by db",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policyA",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						ID:              200,
						Database:        "database2",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
			args: args{
				filter: influxdb.DBRPMappingFilterV2{
					Database: stringPtr("database1"),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policyA",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
				},
			},
		},
		{
			name: "find by rp",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database",
						RetentionPolicy: "retention_policyA",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						ID:              200,
						Database:        "database",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
			args: args{
				filter: influxdb.DBRPMappingFilterV2{
					RetentionPolicy: stringPtr("retention_policyB"),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              200,
						Database:        "database",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
		},
		{
			name: "find by default",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database",
						RetentionPolicy: "retention_policyA",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						ID:              200,
						Database:        "database",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
			args: args{
				filter: influxdb.DBRPMappingFilterV2{
					Default: boolPtr(true),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              200,
						Database:        "database",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
		},
		{
			name: "find default",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              MustIDBase16("0000000000000100"),
						Database:        "database",
						RetentionPolicy: "retention_policyA",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						ID:              MustIDBase16("0000000000000200"),
						Database:        "database",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
					{
						ID:              MustIDBase16("0000000000000300"),
						Database:        "database",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
			args: args{
				filter: influxdb.DBRPMappingFilterV2{
					OrgID:    MustIDBase16Ptr(dbrpOrg3ID),
					Database: stringPtr("database"),
					Default:  boolPtr(true),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              MustIDBase16("0000000000000200"),
						Database:        "database",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
		},
		{
			name: "mixed",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policyA",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						ID:              200,
						Database:        "database2",
						RetentionPolicy: "retention_policyA",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						ID:              300,
						Database:        "database1",
						RetentionPolicy: "retention_policyA",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					// This one will substitute 200 as default for "database2".
					{
						ID:              400,
						Database:        "database2",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
			args: args{
				filter: influxdb.DBRPMappingFilterV2{
					RetentionPolicy: stringPtr("retention_policyA"),
					Default:         boolPtr(true),
					OrgID:           MustIDBase16Ptr(dbrpOrg3ID),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policyA",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
				},
			},
		},
		{
			name: "not found",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						Database:        "database1",
						RetentionPolicy: "retention_policyA",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						Database:        "database1",
						RetentionPolicy: "retention_policyB",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						Database:        "database1",
						RetentionPolicy: "retention_policyA",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						Database:        "database2",
						RetentionPolicy: "retention_policyB",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketBID),
					},
				},
			},
			args: args{
				filter: influxdb.DBRPMappingFilterV2{
					Database:        stringPtr("database1"),
					RetentionPolicy: stringPtr("retention_policyC"),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, done := init(tt.fields, t)
			defer done()
			ctx := context.Background()

			dbrpMappings, _, err := s.FindMany(ctx, tt.args.filter)
			if (err != nil) != (tt.wants.err != nil) {
				t.Fatalf("expected errors to be equal '%v' got '%v'", tt.wants.err, err)
			}

			if err != nil && tt.wants.err != nil {
				if err.Error() != tt.wants.err.Error() {
					t.Fatalf("expected error '%v' got '%v'", tt.wants.err, err)
				}
			}

			if diff := cmp.Diff(dbrpMappings, tt.wants.dbrpMappings, DBRPMappingCmpOptionsV2...); diff != "" {
				t.Errorf("dbrpMappings are different -got/+want\ndiff %s", diff)
			}
		})
	}
}

func FindDBRPMappingByIDV2(
	init func(DBRPMappingFieldsV2, *testing.T) (influxdb.DBRPMappingServiceV2, func()),
	t *testing.T,
) {
	type args struct {
		OrgID platform.ID
		ID    platform.ID
	}

	type wants struct {
		dbrpMapping *influxdb.DBRPMappingV2
		err         error
	}

	tests := []struct {
		name   string
		fields DBRPMappingFieldsV2
		args   args
		wants  wants
	}{
		{
			name: "find existing dbrp",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database",
						RetentionPolicy: "retention_policyA",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						ID:              200,
						Database:        "database",
						RetentionPolicy: "retention_policyB",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
					{
						ID:              300,
						Database:        "database",
						RetentionPolicy: "retention_policyC",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
				},
			},
			args: args{
				OrgID: MustIDBase16(dbrpOrg3ID),
				ID:    200,
			},
			wants: wants{
				dbrpMapping: &influxdb.DBRPMappingV2{
					ID:              200,
					Database:        "database",
					RetentionPolicy: "retention_policyB",
					Default:         false,
					OrganizationID:  MustIDBase16(dbrpOrg3ID),
					BucketID:        MustIDBase16(dbrpBucketAID),
				},
			},
		},
		{
			name: "find non existing dbrp",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database",
						RetentionPolicy: "retention_policyA",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
				},
			},
			args: args{
				OrgID: MustIDBase16(dbrpOrg3ID),
				ID:    200,
			},
			wants: wants{
				err: dbrp.ErrDBRPNotFound,
			},
		},
		{
			name: "find existing dbrp but wrong orgID",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database",
						RetentionPolicy: "retention_policyA",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg3ID),
						BucketID:        MustIDBase16(dbrpBucketAID),
					},
				},
			},
			args: args{
				OrgID: MustIDBase16(dbrpOrg2ID),
				ID:    100,
			},
			wants: wants{
				err: dbrp.ErrDBRPNotFound,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, done := init(tt.fields, t)
			defer done()
			ctx := context.Background()

			dbrpMapping, err := s.FindByID(ctx, tt.args.OrgID, tt.args.ID)
			if (err != nil) != (tt.wants.err != nil) {
				t.Fatalf("expected error '%v' got '%v'", tt.wants.err, err)
			}

			if err != nil && tt.wants.err != nil {
				if err.Error() != tt.wants.err.Error() {
					t.Fatalf("expected error messages to match '%v' got '%v'", tt.wants.err, err.Error())
				}
			}

			if diff := cmp.Diff(dbrpMapping, tt.wants.dbrpMapping, DBRPMappingCmpOptionsV2...); diff != "" {
				t.Errorf("dbrpMappings are different -got/+want\ndiff %s", diff)
			}
		})
	}
}

func UpdateDBRPMappingV2(
	init func(DBRPMappingFieldsV2, *testing.T) (influxdb.DBRPMappingServiceV2, func()),
	t *testing.T,
) {
	type args struct {
		dbrpMapping *influxdb.DBRPMappingV2
	}
	type wants struct {
		err          error
		dbrpMappings []*influxdb.DBRPMappingV2
	}

	tests := []struct {
		name   string
		fields DBRPMappingFieldsV2
		args   args
		wants  wants
	}{
		{
			name: "basic update",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
				},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy2",
					Default:         false,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy2",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
		},
		{
			name: "update invalid dbrp",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         false,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					ID:              100,
					Database:        "./", // invalid db name.
					RetentionPolicy: "retention_policy2",
					Default:         false,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket2ID),
				},
			},
			wants: wants{
				err: dbrp.ErrInvalidDBRP(fmt.Errorf("database must contain at least one character and only be letters, numbers, '_', '-', and '.'")),
				dbrpMappings: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
		},
		{
			name: "error dbrp not found",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         false,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					ID:              200,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket2ID),
				},
			},
			wants: wants{
				err: dbrp.ErrDBRPNotFound,
				dbrpMappings: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
		},
		{
			name: "update unchangeable fields",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         false,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					ID:              100,
					Database:        "wont_change",
					RetentionPolicy: "retention_policy1",
					Default:         false,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket2ID),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
		},
		{
			name: "update to same orgID, db, and rp",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              200,
						Database:        "database1",
						RetentionPolicy: "retention_policy2",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
				},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					ID:              200,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				},
			},
			wants: wants{
				err: dbrp.ErrDBRPAlreadyExists("another DBRP mapping with same orgID, db, and rp exists"),
				dbrpMappings: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         false,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				},
					{
						ID:              200,
						Database:        "database1",
						RetentionPolicy: "retention_policy2",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
				},
			},
		},
		{
			name: "update default when only one dbrp is present",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					ID:              100,
					Database:        "wont_change",
					RetentionPolicy: "retention_policy1",
					Default:         false,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket2ID),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{{
					ID:              100,
					Database:        "database1",
					RetentionPolicy: "retention_policy1",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				}},
			},
		},
		{
			name: "set default when more dbrps are present",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              200,
						Database:        "database1",
						RetentionPolicy: "retention_policy2",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              300,
						Database:        "database1",
						RetentionPolicy: "retention_policy3",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
				},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					ID:              200,
					Database:        "database1",
					RetentionPolicy: "retention_policy2",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              200,
						Database:        "database1",
						RetentionPolicy: "retention_policy2",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              300,
						Database:        "database1",
						RetentionPolicy: "retention_policy3",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
				},
			},
		},
		{
			name: "unset default when more dbrps are present",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              200,
						Database:        "database1",
						RetentionPolicy: "retention_policy2",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              300,
						Database:        "database1",
						RetentionPolicy: "retention_policy3",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
				},
			},
			args: args{
				dbrpMapping: &influxdb.DBRPMappingV2{
					ID:              300,
					Database:        "database1",
					RetentionPolicy: "retention_policy3",
					Default:         false,
					OrganizationID:  MustIDBase16(dbrpOrg1ID),
					BucketID:        MustIDBase16(dbrpBucket1ID),
				},
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              200,
						Database:        "database1",
						RetentionPolicy: "retention_policy2",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              300,
						Database:        "database1",
						RetentionPolicy: "retention_policy3",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, done := init(tt.fields, t)
			defer done()
			ctx := context.Background()
			err := s.Update(ctx, tt.args.dbrpMapping)
			if (err != nil) != (tt.wants.err != nil) {
				t.Errorf("expected error '%v' got '%v'", tt.wants.err, err)
			}

			if err != nil && tt.wants.err != nil {
				if err.Error() != tt.wants.err.Error() {
					t.Errorf("expected error messages to match '%v' got '%v'", tt.wants.err, err.Error())
				}
			}

			dbrpMappings, _, err := s.FindMany(ctx, influxdb.DBRPMappingFilterV2{})
			if err != nil {
				t.Fatalf("failed to retrieve dbrps: %v", err)
			}
			if diff := cmp.Diff(dbrpMappings, tt.wants.dbrpMappings, DBRPMappingCmpOptionsV2...); diff != "" {
				t.Errorf("dbrpMappings are different -got/+want\ndiff %s", diff)
			}
		})
	}
}

func DeleteDBRPMappingV2(
	init func(DBRPMappingFieldsV2, *testing.T) (influxdb.DBRPMappingServiceV2, func()),
	t *testing.T,
) {
	type args struct {
		OrgID platform.ID
		ID    platform.ID
	}
	type wants struct {
		err          error
		dbrpMappings []*influxdb.DBRPMappingV2
	}

	tests := []struct {
		name   string
		fields DBRPMappingFieldsV2
		args   args
		wants  wants
	}{
		{
			name: "delete existing dbrp",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              200,
						Database:        "database2",
						RetentionPolicy: "retention_policy2",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucket2ID),
					},
				},
			},
			args: args{
				OrgID: MustIDBase16(dbrpOrg1ID),
				ID:    100,
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{{
					ID:              200,
					Database:        "database2",
					RetentionPolicy: "retention_policy2",
					Default:         true,
					OrganizationID:  MustIDBase16(dbrpOrg2ID),
					BucketID:        MustIDBase16(dbrpBucket2ID),
				}},
			},
		},
		{
			name: "delete default dbrp",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              200,
						Database:        "database1",
						RetentionPolicy: "retention_policy2",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              300,
						Database:        "database1",
						RetentionPolicy: "retention_policy3",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              400,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              500,
						Database:        "database1",
						RetentionPolicy: "retention_policy2",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
				},
			},
			args: args{
				OrgID: MustIDBase16(dbrpOrg1ID),
				ID:    200,
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					// The first one becomes the default one.
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              300,
						Database:        "database1",
						RetentionPolicy: "retention_policy3",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              400,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              500,
						Database:        "database1",
						RetentionPolicy: "retention_policy2",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
				},
			},
		},
		{
			name: "delete non-existing dbrp",
			fields: DBRPMappingFieldsV2{
				DBRPMappingsV2: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         false,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              200,
						Database:        "database2",
						RetentionPolicy: "retention_policy2",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucket2ID),
					},
				},
			},
			args: args{
				OrgID: MustIDBase16(dbrpOrg2ID),
				ID:    100,
			},
			wants: wants{
				dbrpMappings: []*influxdb.DBRPMappingV2{
					{
						ID:              100,
						Database:        "database1",
						RetentionPolicy: "retention_policy1",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg1ID),
						BucketID:        MustIDBase16(dbrpBucket1ID),
					},
					{
						ID:              200,
						Database:        "database2",
						RetentionPolicy: "retention_policy2",
						Default:         true,
						OrganizationID:  MustIDBase16(dbrpOrg2ID),
						BucketID:        MustIDBase16(dbrpBucket2ID),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, done := init(tt.fields, t)
			defer done()
			ctx := context.Background()
			err := s.Delete(ctx, tt.args.OrgID, tt.args.ID)
			if (err != nil) != (tt.wants.err != nil) {
				t.Fatalf("expected error '%v' got '%v'", tt.wants.err, err)
			}

			if err != nil && tt.wants.err != nil {
				if err.Error() != tt.wants.err.Error() {
					t.Fatalf("expected error messages to match '%v' got '%v'", tt.wants.err, err.Error())
				}
			}

			filter := influxdb.DBRPMappingFilterV2{}
			dbrpMappings, _, err := s.FindMany(ctx, filter)
			if err != nil {
				t.Fatalf("failed to retrieve dbrps: %v", err)
			}
			if diff := cmp.Diff(dbrpMappings, tt.wants.dbrpMappings, DBRPMappingCmpOptionsV2...); diff != "" {
				t.Errorf("dbrpMappings are different -got/+want\ndiff %s", diff)
			}
		})
	}
}

func MiscDBRPMappingV2(
	init func(DBRPMappingFieldsV2, *testing.T) (influxdb.DBRPMappingServiceV2, func()),
	t *testing.T,
) {
	fields := DBRPMappingFieldsV2{
		DBRPMappingsV2: []*influxdb.DBRPMappingV2{
			{
				ID:              100,
				Database:        "database1",
				RetentionPolicy: "retention_policy1",
				Default:         false,
				OrganizationID:  MustIDBase16(dbrpOrg1ID),
				BucketID:        MustIDBase16(dbrpBucket1ID),
			},
			{
				ID:              200,
				Database:        "database2",
				RetentionPolicy: "retention_policy2",
				Default:         true,
				OrganizationID:  MustIDBase16(dbrpOrg2ID),
				BucketID:        MustIDBase16(dbrpBucket2ID),
			},
		},
	}
	s, done := init(fields, t)
	defer done()
	ctx := context.Background()

	t.Run("defaults are ok", func(t *testing.T) {
		if !fields.DBRPMappingsV2[0].Default || !fields.DBRPMappingsV2[1].Default {
			t.Errorf("should be default")
		}
	})

	t.Run("what is inited is present", func(t *testing.T) {
		filter := influxdb.DBRPMappingFilterV2{}
		dbrpMappings, _, err := s.FindMany(ctx, filter)
		if err != nil {
			t.Fatalf("failed to retrieve dbrps: %v", err)
		}
		if diff := cmp.Diff(dbrpMappings, fields.DBRPMappingsV2, DBRPMappingCmpOptionsV2...); diff != "" {
			t.Errorf("dbrpMappings are different -got/+want\ndiff %s", diff)
		}
	})

	t.Run("delete works", func(t *testing.T) {
		err := s.Delete(ctx, fields.DBRPMappingsV2[0].OrganizationID, fields.DBRPMappingsV2[0].ID)
		if err != nil {
			t.Fatalf("failed to delete: %v", err)
		}
		err = s.Delete(ctx, fields.DBRPMappingsV2[1].OrganizationID, fields.DBRPMappingsV2[1].ID)
		if err != nil {
			t.Fatalf("failed to delete: %v", err)
		}
	})

	t.Run("nothing left", func(t *testing.T) {
		filter := influxdb.DBRPMappingFilterV2{}
		dbrpMappings, _, err := s.FindMany(ctx, filter)
		if err != nil {
			t.Fatalf("failed to retrieve dbrps: %v", err)
		}
		if diff := cmp.Diff(dbrpMappings, []*influxdb.DBRPMappingV2{}, DBRPMappingCmpOptionsV2...); diff != "" {
			t.Errorf("dbrpMappings are different -got/+want\ndiff %s", diff)
		}
	})

	t.Run("new one is still ok", func(t *testing.T) {
		m := &influxdb.DBRPMappingV2{
			ID:              300,
			Database:        "database2",
			RetentionPolicy: "retention_policy2",
			Default:         false,
			OrganizationID:  MustIDBase16(dbrpOrg2ID),
			BucketID:        MustIDBase16(dbrpBucket2ID),
		}
		if err := s.Create(ctx, m); err != nil {
			t.Fatalf("failed to create: %v", err)
		}
		got, err := s.FindByID(ctx, m.OrganizationID, m.ID)
		if err != nil {
			t.Fatalf("failed to retrieve dbrp: %v", err)
		}
		if diff := cmp.Diff(m, got, DBRPMappingCmpOptionsV2...); diff != "" {
			t.Errorf("dbrpMappings are different -got/+want\ndiff %s", diff)
		}
		if !m.Default {
			t.Errorf("should be default")
		}
	})
}
