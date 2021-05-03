package launcher_test

import (
	"context"
	"github.com/influxdata/influxdb/v2/kit/platform/errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/influxdata/influxdb/v2"
	"github.com/influxdata/influxdb/v2/backup"
	"github.com/influxdata/influxdb/v2/cmd/influxd/launcher"
	"github.com/influxdata/influxdb/v2/restore"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestBackupRestore_Full(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	backupDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer os.RemoveAll(backupDir)

	// Boot a server, write some data, and take a backup.
	l1 := launcher.RunAndSetupNewLauncherOrFail(ctx, t, func(o *launcher.InfluxdOpts) {
		o.StoreType = "bolt"
		o.Testing = false
		o.LogLevel = zap.InfoLevel
	})
	l1.WritePointsOrFail(t, "m,k=v1 f=100i 946684800000000000\nm,k=v2 f=200i 946684800000000001")
	l1.BackupOrFail(t, ctx, backup.Request{Path: backupDir})

	// Create a new bucket, write data into it (+ the old bucket), and take another backup.
	b1 := influxdb.Bucket{OrgID: l1.Org.ID, Name: "bucket2"}
	require.NoError(t, l1.BucketService(t).CreateBucket(ctx, &b1))
	l1.WriteOrFail(t, &influxdb.OnboardingResults{
		Org:    l1.Org,
		Bucket: &b1,
		Auth:   l1.Auth,
	}, "m,k=v1 f=100i 946684800000000005\nm,k=v2 f=200i 946684800000000006")
	l1.WritePointsOrFail(t, "m,k=v1 f=100i 946684800000000002\nm,k=v2 f=200i 946684800000000003")
	l1.BackupOrFail(t, ctx, backup.Request{Path: backupDir})

	// Shut down the server.
	l1.ShutdownOrFail(t, ctx)

	// Boot up a second server, using the same auth token as the previous.
	l2 := launcher.NewTestLauncher()
	l2.RunOrFail(t, ctx, func(o *launcher.InfluxdOpts) {
		o.StoreType = "bolt"
		o.Testing = false
		o.LogLevel = zap.InfoLevel
	})
	defer l2.ShutdownOrFail(t, ctx)

	onboardReq := influxdb.OnboardingRequest{
		User:     "USER",
		Password: "PASSWORD",
		Org:      "ORG",
		Bucket:   "BUCKET",
		Token:    l1.Auth.Token,
	}
	onboardRes := l2.OnBoardOrFail(t, &onboardReq)
	l2.Org = onboardRes.Org
	l2.Bucket = onboardRes.Bucket
	l2.Auth = onboardRes.Auth

	// Create a second bucket, write data into it.
	b2 := influxdb.Bucket{OrgID: onboardRes.Org.ID, Name: "2bucket"}
	require.NoError(t, l2.BucketService(t).CreateBucket(ctx, &b2))
	l2.WriteOrFail(t, &influxdb.OnboardingResults{
		Org:    onboardRes.Org,
		Bucket: &b2,
		Auth:   onboardRes.Auth,
	}, "m,k=v5 f=100i 946684800000000005\nm,k=v7 f=200i 946684800000000006")

	// Perform a full restore from the previous backups.
	l2.RestoreOrFail(t, ctx, restore.Request{Path: backupDir, Full: true})

	// Check that orgs and buckets were reset to match the original server's metadata.
	_, err = l2.OrgService(t).FindOrganizationByID(ctx, l2.Org.ID)
	require.Equal(t, errors.ENotFound, errors.ErrorCode(err))
	rbkt1, err := l2.BucketService(t).FindBucket(ctx, influxdb.BucketFilter{OrganizationID: &l1.Org.ID, ID: &l1.Bucket.ID})
	require.NoError(t, err)
	require.Equal(t, l1.Bucket.Name, rbkt1.Name)
	rbkt2, err := l2.BucketService(t).FindBucket(ctx, influxdb.BucketFilter{OrganizationID: &l1.Org.ID, ID: &b1.ID})
	require.NoError(t, err)
	require.Equal(t, b1.Name, rbkt2.Name)
	_, err = l2.BucketService(t).FindBucket(ctx, influxdb.BucketFilter{OrganizationID: &l2.Org.ID, ID: &b2.ID})
	require.Equal(t, errors.ENotFound, errors.ErrorCode(err))

	// Check that data was restored to buckets.
	q1 := `from(bucket:"BUCKET") |> range(start:2000-01-01T00:00:00Z,stop:2000-01-02T00:00:00Z)`
	exp1 := `,result,table,_start,_stop,_time,_value,_field,_measurement,k` + "\r\n" +
		`,_result,0,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00Z,100,f,m,v1` + "\r\n" +
		`,_result,0,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00.000000002Z,100,f,m,v1` + "\r\n" +
		`,_result,1,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00.000000001Z,200,f,m,v2` + "\r\n" +
		`,_result,1,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00.000000003Z,200,f,m,v2` + "\r\n\r\n"
	res1 := l2.FluxQueryOrFail(t, l2.Org, l2.Auth.Token, q1)
	require.Equal(t, exp1, res1)

	q2 := `from(bucket:"bucket2") |> range(start:2000-01-01T00:00:00Z,stop:2000-01-02T00:00:00Z)`
	exp2 := `,result,table,_start,_stop,_time,_value,_field,_measurement,k` + "\r\n" +
		`,_result,0,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00.000000005Z,100,f,m,v1` + "\r\n" +
		`,_result,1,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00.000000006Z,200,f,m,v2` + "\r\n\r\n"
	res2 := l2.FluxQueryOrFail(t, l2.Org, l2.Auth.Token, q2)
	require.Equal(t, exp2, res2)
}

func TestBackupRestore_Partial(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	backupDir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	defer os.RemoveAll(backupDir)

	// Boot a server, write some data, and take a backup.
	l1 := launcher.RunAndSetupNewLauncherOrFail(ctx, t, func(o *launcher.InfluxdOpts) {
		o.StoreType = "bolt"
		o.Testing = false
		o.LogLevel = zap.InfoLevel
	})
	l1.WritePointsOrFail(t, "m,k=v1 f=100i 946684800000000000\nm,k=v2 f=200i 946684800000000001")
	l1.BackupOrFail(t, ctx, backup.Request{Path: backupDir})

	// Create a new bucket, write data into it (+ the old bucket), and take another backup.
	b1 := influxdb.Bucket{OrgID: l1.Org.ID, Name: "bucket2"}
	require.NoError(t, l1.BucketService(t).CreateBucket(ctx, &b1))
	l1.WriteOrFail(t, &influxdb.OnboardingResults{
		Org:    l1.Org,
		Bucket: &b1,
		Auth:   l1.Auth,
	}, "m,k=v1 f=100i 946684800000000005\nm,k=v2 f=200i 946684800000000006")
	l1.WritePointsOrFail(t, "m,k=v1 f=100i 946684800000000002\nm,k=v2 f=200i 946684800000000003")
	l1.BackupOrFail(t, ctx, backup.Request{Path: backupDir})

	// Shut down the server.
	l1.ShutdownOrFail(t, ctx)

	// Boot up a second server.
	l2 := launcher.NewTestLauncher()
	l2.RunOrFail(t, ctx, func(o *launcher.InfluxdOpts) {
		o.StoreType = "bolt"
		o.Testing = false
		o.LogLevel = zap.InfoLevel
	})
	defer l2.ShutdownOrFail(t, ctx)

	onboardReq := influxdb.OnboardingRequest{
		User:     "USER",
		Password: "PASSWORD",
		Org:      "ORG2",
		Bucket:   "BUCKET",
	}
	onboardRes := l2.OnBoardOrFail(t, &onboardReq)
	l2.Org = onboardRes.Org
	l2.Bucket = onboardRes.Bucket
	l2.Auth = onboardRes.Auth

	// Create a second bucket, write data into it.
	b2 := influxdb.Bucket{OrgID: onboardRes.Org.ID, Name: "2bucket"}
	require.NoError(t, l2.BucketService(t).CreateBucket(ctx, &b2))
	l2.WriteOrFail(t, &influxdb.OnboardingResults{
		Org:    onboardRes.Org,
		Bucket: &b2,
		Auth:   onboardRes.Auth,
	}, "m,k=v5 f=100i 946684800000000005\nm,k=v7 f=200i 946684800000000006")

	// Perform a partial restore from the previous backups.
	l2.RestoreOrFail(t, ctx, restore.Request{Path: backupDir})

	// Check that buckets from the 1st launcher were restored to the new server.
	rbkt1, err := l2.BucketService(t).FindBucket(ctx, influxdb.BucketFilter{Org: &l1.Org.Name, Name: &l1.Bucket.Name})
	require.NoError(t, err)
	require.Equal(t, l1.Bucket.Name, rbkt1.Name)
	rbkt2, err := l2.BucketService(t).FindBucket(ctx, influxdb.BucketFilter{Org: &l1.Org.Name, Name: &b1.Name})
	require.NoError(t, err)
	require.Equal(t, b1.Name, rbkt2.Name)

	// Check that data was restored to buckets.
	q1 := `from(bucket:"BUCKET") |> range(start:2000-01-01T00:00:00Z,stop:2000-01-02T00:00:00Z)`
	exp1 := `,result,table,_start,_stop,_time,_value,_field,_measurement,k` + "\r\n" +
		`,_result,0,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00Z,100,f,m,v1` + "\r\n" +
		`,_result,0,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00.000000002Z,100,f,m,v1` + "\r\n" +
		`,_result,1,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00.000000001Z,200,f,m,v2` + "\r\n" +
		`,_result,1,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00.000000003Z,200,f,m,v2` + "\r\n\r\n"
	res1 := l2.FluxQueryOrFail(t, l1.Org, l2.Auth.Token, q1)
	require.Equal(t, exp1, res1)

	q2 := `from(bucket:"bucket2") |> range(start:2000-01-01T00:00:00Z,stop:2000-01-02T00:00:00Z)`
	exp2 := `,result,table,_start,_stop,_time,_value,_field,_measurement,k` + "\r\n" +
		`,_result,0,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00.000000005Z,100,f,m,v1` + "\r\n" +
		`,_result,1,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00.000000006Z,200,f,m,v2` + "\r\n\r\n"
	res2 := l2.FluxQueryOrFail(t, l1.Org, l2.Auth.Token, q2)
	require.Equal(t, exp2, res2)

	// Check that the 2nd launcher's buckets weren't touched.
	newBucket1, err := l2.BucketService(t).FindBucket(ctx, influxdb.BucketFilter{OrganizationID: &l2.Org.ID, ID: &l2.Bucket.ID})
	require.NoError(t, err)
	require.Equal(t, l2.Bucket.Name, newBucket1.Name)
	newBucket2, err := l2.BucketService(t).FindBucket(ctx, influxdb.BucketFilter{OrganizationID: &l2.Org.ID, ID: &b2.ID})
	require.NoError(t, err)
	require.Equal(t, b2.Name, newBucket2.Name)

	q3 := `from(bucket:"2bucket") |> range(start:2000-01-01T00:00:00Z,stop:2000-01-02T00:00:00Z)`
	exp3 := `,result,table,_start,_stop,_time,_value,_field,_measurement,k` + "\r\n" +
		`,_result,0,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00.000000005Z,100,f,m,v5` + "\r\n" +
		`,_result,1,2000-01-01T00:00:00Z,2000-01-02T00:00:00Z,2000-01-01T00:00:00.000000006Z,200,f,m,v7` + "\r\n\r\n"
	res3 := l2.FluxQueryOrFail(t, l2.Org, l2.Auth.Token, q3)
	require.Equal(t, exp3, res3)
}
