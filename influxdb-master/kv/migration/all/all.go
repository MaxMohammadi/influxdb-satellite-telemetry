package all

import (
	"github.com/influxdata/influxdb/v2/kv/migration"
)

// Migrations contains all the migrations required for the entire of the
// kv store backing influxdb's metadata.
var Migrations = [...]migration.Spec{
	// initial migrations
	Migration0001_InitialMigration,
	// add index user resource mappings by user id
	Migration0002_AddURMByUserIndex,
	// add index for tasks with missing owner IDs
	Migration0003_TaskOwnerIDUpMigration,
	// add dbrp buckets
	Migration0004_AddDbrpBuckets,
	// add pkger buckets
	Migration0005_AddPkgerBuckets,
	// delete bucket sessionsv1
	Migration0006_DeleteBucketSessionsv1,
	// CreateMetaDataBucket
	Migration0007_CreateMetaDataBucket,
	// LegacyAuthBuckets
	Migration0008_LegacyAuthBuckets,
	// LegacyAuthPasswordBuckets
	Migration0009_LegacyAuthPasswordBuckets,
	// add index telegraf by org
	Migration0010_AddIndexTelegrafByOrg,
	// populate dashboards owner id
	Migration0011_PopulateDashboardsOwnerId,
	// Populate the DBRP service ByOrg index
	Migration0012_DBRPByOrgIndex,
	// repair DBRP owner and bucket IDs
	Migration0013_RepairDBRPOwnerAndBucketIDs,
	// reindex DBRPs
	Migration0014_ReindexDBRPs,
	// record shard group durations in bucket metadata
	Migration0015_RecordShardGroupDurationsInBucketMetadata,
	// {{ do_not_edit . }}
}
