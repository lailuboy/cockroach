// Code generated by "stringer -type=VersionKey"; DO NOT EDIT.

package cluster

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[Version2_1-0]
	_ = x[VersionCascadingZoneConfigs-1]
	_ = x[VersionLoadSplits-2]
	_ = x[VersionExportStorageWorkload-3]
	_ = x[VersionLazyTxnRecord-4]
	_ = x[VersionSequencedReads-5]
	_ = x[VersionUnreplicatedRaftTruncatedState-6]
	_ = x[VersionCreateStats-7]
	_ = x[VersionDirectImport-8]
	_ = x[VersionSideloadedStorageNoReplicaID-9]
	_ = x[VersionPushTxnToInclusive-10]
	_ = x[VersionSnapshotsWithoutLog-11]
	_ = x[Version19_1-12]
	_ = x[VersionStart19_2-13]
	_ = x[VersionQueryTxnTimestamp-14]
	_ = x[VersionStickyBit-15]
	_ = x[VersionParallelCommits-16]
}

const _VersionKey_name = "Version2_1VersionCascadingZoneConfigsVersionLoadSplitsVersionExportStorageWorkloadVersionLazyTxnRecordVersionSequencedReadsVersionUnreplicatedRaftTruncatedStateVersionCreateStatsVersionDirectImportVersionSideloadedStorageNoReplicaIDVersionPushTxnToInclusiveVersionSnapshotsWithoutLogVersion19_1VersionStart19_2VersionQueryTxnTimestampVersionStickyBitVersionParallelCommits"

var _VersionKey_index = [...]uint16{0, 10, 37, 54, 82, 102, 123, 160, 178, 197, 232, 257, 283, 294, 310, 334, 350, 372}

func (i VersionKey) String() string {
	if i < 0 || i >= VersionKey(len(_VersionKey_index)-1) {
		return "VersionKey(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _VersionKey_name[_VersionKey_index[i]:_VersionKey_index[i+1]]
}
