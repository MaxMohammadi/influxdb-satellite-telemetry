package testing

import (
	"context"
	"strings"
	"testing"

	"github.com/influxdata/influxdb/v2/kit/platform"
	"github.com/influxdata/influxdb/v2/kit/platform/errors"

	"github.com/influxdata/influxdb/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TODO(goller): remove opPrefix argument
func diffPlatformErrors(name string, actual, expected error, opPrefix string, t *testing.T) {
	t.Helper()
	ErrorsEqual(t, actual, expected)
}

// ErrorsEqual checks to see if the provided errors are equivalent.
func ErrorsEqual(t *testing.T, actual, expected error) {
	t.Helper()
	if expected == nil && actual == nil {
		return
	}

	if expected == nil && actual != nil {
		t.Errorf("unexpected error %s", actual.Error())
	}

	if expected != nil && actual == nil {
		t.Errorf("expected error %s but received nil", expected.Error())
	}

	if errors.ErrorCode(expected) != errors.ErrorCode(actual) {
		t.Logf("\nexpected: %v\nactual: %v\n\n", expected, actual)
		t.Errorf("expected error code %q but received %q", errors.ErrorCode(expected), errors.ErrorCode(actual))
	}

	if errors.ErrorMessage(expected) != errors.ErrorMessage(actual) {
		t.Logf("\nexpected: %v\nactual: %v\n\n", expected, actual)
		t.Errorf("expected error message %q but received %q", errors.ErrorMessage(expected), errors.ErrorMessage(actual))
	}
}

// FloatPtr takes the ref of a float number.
func FloatPtr(f float64) *float64 {
	p := new(float64)
	*p = f
	return p
}

func idPtr(id platform.ID) *platform.ID {
	return &id
}

// MustIDBase16 is an helper to ensure a correct ID is built during testing.
func MustIDBase16(s string) platform.ID {
	id, err := platform.IDFromString(s)
	if err != nil {
		panic(err)
	}
	return *id
}

// MustIDBase16Ptr is an helper to ensure a correct ID ptr ref is built during testing.
func MustIDBase16Ptr(s string) *platform.ID {
	id := MustIDBase16(s)
	return &id
}

func MustCreateOrgs(ctx context.Context, svc influxdb.OrganizationService, os ...*influxdb.Organization) {
	for _, o := range os {
		if err := svc.CreateOrganization(ctx, o); err != nil {
			panic(err)
		}
	}
}

func MustCreateLabels(ctx context.Context, svc influxdb.LabelService, labels ...*influxdb.Label) {
	for _, l := range labels {
		if err := svc.CreateLabel(ctx, l); err != nil {
			panic(err)
		}
	}
}

func MustCreateUsers(ctx context.Context, svc influxdb.UserService, us ...*influxdb.User) {
	for _, u := range us {
		if err := svc.CreateUser(ctx, u); err != nil {
			panic(err)
		}
	}
}

func MustCreateMappings(ctx context.Context, svc influxdb.UserResourceMappingService, ms ...*influxdb.UserResourceMapping) {
	for _, m := range ms {
		if err := svc.CreateUserResourceMapping(ctx, m); err != nil {
			panic(err)
		}
	}
}

func MustMakeUsersOrgOwner(ctx context.Context, svc influxdb.UserResourceMappingService, oid platform.ID, uids ...platform.ID) {
	ms := make([]*influxdb.UserResourceMapping, len(uids))
	for i, uid := range uids {
		ms[i] = &influxdb.UserResourceMapping{
			UserID:       uid,
			UserType:     influxdb.Owner,
			ResourceType: influxdb.OrgsResourceType,
			ResourceID:   oid,
		}
	}
	MustCreateMappings(ctx, svc, ms...)
}

func MustMakeUsersOrgMember(ctx context.Context, svc influxdb.UserResourceMappingService, oid platform.ID, uids ...platform.ID) {
	ms := make([]*influxdb.UserResourceMapping, len(uids))
	for i, uid := range uids {
		ms[i] = &influxdb.UserResourceMapping{
			UserID:       uid,
			UserType:     influxdb.Member,
			ResourceType: influxdb.OrgsResourceType,
			ResourceID:   oid,
		}
	}
	MustCreateMappings(ctx, svc, ms...)
}

func MustNewPermissionAtID(id platform.ID, a influxdb.Action, rt influxdb.ResourceType, orgID platform.ID) *influxdb.Permission {
	perm, err := influxdb.NewPermissionAtID(id, a, rt, orgID)
	if err != nil {
		panic(err)
	}
	return perm
}

func influxErrsEqual(t *testing.T, expected *errors.Error, actual error) {
	t.Helper()

	if expected != nil {
		require.Error(t, actual)
	}

	if actual == nil {
		return
	}

	if expected == nil {
		require.NoError(t, actual)
		return
	}
	iErr, ok := actual.(*errors.Error)
	require.True(t, ok)
	assert.Equal(t, expected.Code, iErr.Code)
	assert.Truef(t, strings.HasPrefix(iErr.Error(), expected.Error()), "expected: %s got err: %s", expected.Error(), actual.Error())
}
