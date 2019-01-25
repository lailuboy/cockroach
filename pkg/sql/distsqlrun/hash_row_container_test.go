// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package distsqlrun

import (
	"context"
	"math"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlbase"
	"github.com/cockroachdb/cockroach/pkg/storage/engine"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/mon"
)

func TestHashDiskBackedRowContainer(t *testing.T) {
	defer leaktest.AfterTest(t)()

	ctx := context.Background()
	st := cluster.MakeTestingClusterSettings()
	evalCtx := tree.MakeTestingEvalContext(st)
	tempEngine, err := engine.NewTempEngine(base.DefaultTestTempStorageConfig(st), base.DefaultTestStoreSpec)
	if err != nil {
		t.Fatal(err)
	}
	defer tempEngine.Close()

	// These monitors are started and stopped by subtests.
	memoryMonitor := mon.MakeMonitor(
		"test-mem",
		mon.MemoryResource,
		nil,           /* curCount */
		nil,           /* maxHist */
		-1,            /* increment */
		math.MaxInt64, /* noteworthy */
		st,
	)
	diskMonitor := mon.MakeMonitor(
		"test-disk",
		mon.DiskResource,
		nil,           /* curCount */
		nil,           /* maxHist */
		-1,            /* increment */
		math.MaxInt64, /* noteworthy */
		st,
	)

	const numRows = 10
	const numCols = 1
	rows := makeIntRows(numRows, numCols)
	storedEqColumns := columns{0}
	types := []sqlbase.ColumnType{intType}
	ordering := sqlbase.ColumnOrdering{{ColIdx: 0, Direction: encoding.Ascending}}

	rc := makeHashDiskBackedRowContainer(nil, &evalCtx, &memoryMonitor, &diskMonitor, tempEngine)
	err = rc.Init(
		ctx,
		false, /* shouldMark */
		types,
		storedEqColumns,
		true, /*encodeNull */
	)
	if err != nil {
		t.Fatalf("unexpected error while initializing hashDiskBackedRowContainer: %s", err.Error())
	}
	defer rc.Close(ctx)

	// NormalRun adds rows to a hashDiskBackedRowContainer, makes it spill to
	// disk halfway through, keeps on adding rows, and then verifies that all
	// rows were properly added to the hashDiskBackedRowContainer.
	t.Run("NormalRun", func(t *testing.T) {
		memoryMonitor.Start(ctx, nil, mon.MakeStandaloneBudget(math.MaxInt64))
		defer memoryMonitor.Stop(ctx)
		diskMonitor.Start(ctx, nil, mon.MakeStandaloneBudget(math.MaxInt64))
		defer diskMonitor.Stop(ctx)

		defer func() {
			if err := rc.UnsafeReset(ctx); err != nil {
				t.Fatal(err)
			}
		}()

		mid := len(rows) / 2
		for i := 0; i < mid; i++ {
			if err := rc.AddRow(ctx, rows[i]); err != nil {
				t.Fatal(err)
			}
		}
		if rc.UsingDisk() {
			t.Fatal("unexpectedly using disk")
		}
		func() {
			// We haven't marked any rows, so the unmarked iterator should iterate
			// over all rows added so far.
			i := rc.NewUnmarkedIterator(ctx)
			defer i.Close()
			if err := verifyRows(ctx, i, rows[:mid], &evalCtx, ordering); err != nil {
				t.Fatalf("verifying memory rows failed with: %s", err)
			}
		}()
		if err := rc.spillToDisk(ctx); err != nil {
			t.Fatal(err)
		}
		if !rc.UsingDisk() {
			t.Fatal("unexpectedly using memory")
		}
		for i := mid; i < len(rows); i++ {
			if err := rc.AddRow(ctx, rows[i]); err != nil {
				t.Fatal(err)
			}
		}
		func() {
			i := rc.NewUnmarkedIterator(ctx)
			defer i.Close()
			if err := verifyRows(ctx, i, rows, &evalCtx, ordering); err != nil {
				t.Fatalf("verifying disk rows failed with: %s", err)
			}
		}()
	})

	t.Run("AddRowOutOfMem", func(t *testing.T) {
		memoryMonitor.Start(ctx, nil, mon.MakeStandaloneBudget(1))
		defer memoryMonitor.Stop(ctx)
		diskMonitor.Start(ctx, nil, mon.MakeStandaloneBudget(math.MaxInt64))
		defer diskMonitor.Stop(ctx)

		defer func() {
			if err := rc.UnsafeReset(ctx); err != nil {
				t.Fatal(err)
			}
		}()

		if err := rc.AddRow(ctx, rows[0]); err != nil {
			t.Fatal(err)
		}
		if !rc.UsingDisk() {
			t.Fatal("expected to have spilled to disk")
		}
		if diskMonitor.AllocBytes() == 0 {
			t.Fatal("disk monitor reports no disk usage")
		}
		if memoryMonitor.AllocBytes() > 0 {
			t.Fatal("memory monitor reports unexpected usage")
		}
	})

	t.Run("AddRowOutOfDisk", func(t *testing.T) {
		memoryMonitor.Start(ctx, nil, mon.MakeStandaloneBudget(1))
		defer memoryMonitor.Stop(ctx)
		diskMonitor.Start(ctx, nil, mon.MakeStandaloneBudget(1))

		defer func() {
			if err := rc.UnsafeReset(ctx); err != nil {
				t.Fatal(err)
			}
		}()

		err := rc.AddRow(ctx, rows[0])
		if pgErr, ok := pgerror.GetPGCause(err); !(ok && pgErr.Code == pgerror.CodeDiskFullError) {
			t.Fatalf(
				"unexpected error %v, expected disk full error %s", err, pgerror.CodeDiskFullError,
			)
		}
		if !rc.UsingDisk() {
			t.Fatal("expected to have tried to spill to disk")
		}
		if diskMonitor.AllocBytes() != 0 {
			t.Fatal("disk monitor reports unexpected usage")
		}
		if memoryMonitor.AllocBytes() != 0 {
			t.Fatal("memory monitor reports unexpected usage")
		}
	})
}

func TestHashDiskBackedRowContainerPreservesMatchesAndMarks(t *testing.T) {
	defer leaktest.AfterTest(t)()

	ctx := context.Background()
	st := cluster.MakeTestingClusterSettings()
	evalCtx := tree.MakeTestingEvalContext(st)
	tempEngine, err := engine.NewTempEngine(base.DefaultTestTempStorageConfig(st), base.DefaultTestStoreSpec)
	if err != nil {
		t.Fatal(err)
	}
	defer tempEngine.Close()

	// These monitors are started and stopped by subtests.
	memoryMonitor := mon.MakeMonitor(
		"test-mem",
		mon.MemoryResource,
		nil,           /* curCount */
		nil,           /* maxHist */
		-1,            /* increment */
		math.MaxInt64, /* noteworthy */
		st,
	)
	diskMonitor := mon.MakeMonitor(
		"test-disk",
		mon.DiskResource,
		nil,           /* curCount */
		nil,           /* maxHist */
		-1,            /* increment */
		math.MaxInt64, /* noteworthy */
		st,
	)

	const numRowsInBucket = 4
	const numRows = 12
	const numCols = 2
	rows := makeRepeatedIntRows(numRowsInBucket, numRows, numCols)
	storedEqColumns := columns{0}
	types := []sqlbase.ColumnType{intType, intType}
	ordering := sqlbase.ColumnOrdering{{ColIdx: 0, Direction: encoding.Ascending}}

	rc := makeHashDiskBackedRowContainer(nil, &evalCtx, &memoryMonitor, &diskMonitor, tempEngine)
	err = rc.Init(
		ctx,
		true, /* shouldMark */
		types,
		storedEqColumns,
		true, /*encodeNull */
	)
	if err != nil {
		t.Fatalf("unexpected error while initializing hashDiskBackedRowContainer: %s", err.Error())
	}
	defer rc.Close(ctx)

	// PreservingMatches adds rows from three different buckets to a
	// hashDiskBackedRowContainer, makes it spill to disk, keeps on adding rows
	// from the same buckets, and then verifies that all rows were properly added
	// to the hashDiskBackedRowContainer.
	t.Run("PreservingMatches", func(t *testing.T) {
		memoryMonitor.Start(ctx, nil, mon.MakeStandaloneBudget(math.MaxInt64))
		defer memoryMonitor.Stop(ctx)
		diskMonitor.Start(ctx, nil, mon.MakeStandaloneBudget(math.MaxInt64))
		defer diskMonitor.Stop(ctx)

		defer func() {
			if err := rc.UnsafeReset(ctx); err != nil {
				t.Fatal(err)
			}
		}()

		mid := len(rows) / 2
		for i := 0; i < mid; i++ {
			if err := rc.AddRow(ctx, rows[i]); err != nil {
				t.Fatal(err)
			}
		}
		if rc.UsingDisk() {
			t.Fatal("unexpectedly using disk")
		}
		func() {
			// We haven't marked any rows, so the unmarked iterator should iterate
			// over all rows added so far.
			i := rc.NewUnmarkedIterator(ctx)
			defer i.Close()
			if err := verifyRows(ctx, i, rows[:mid], &evalCtx, ordering); err != nil {
				t.Fatalf("verifying memory rows failed with: %s", err)
			}
		}()
		if err := rc.spillToDisk(ctx); err != nil {
			t.Fatal(err)
		}
		if !rc.UsingDisk() {
			t.Fatal("unexpectedly using memory")
		}
		for i := mid; i < len(rows); i++ {
			if err := rc.AddRow(ctx, rows[i]); err != nil {
				t.Fatal(err)
			}
		}
		func() {
			i := rc.NewUnmarkedIterator(ctx)
			defer i.Close()
			if err := verifyRows(ctx, i, rows, &evalCtx, ordering); err != nil {
				t.Fatalf("verifying disk rows failed with: %s", err)
			}
		}()
	})

	// PreservingMarks adds rows from three buckets to a
	// hashDiskBackedRowContainer, marks all rows belonging to the first bucket,
	// spills to disk, and checks that marks are preserved correctly.
	t.Run("PreservingMarks", func(t *testing.T) {
		memoryMonitor.Start(ctx, nil, mon.MakeStandaloneBudget(math.MaxInt64))
		defer memoryMonitor.Stop(ctx)
		diskMonitor.Start(ctx, nil, mon.MakeStandaloneBudget(math.MaxInt64))
		defer diskMonitor.Stop(ctx)

		defer func() {
			if err := rc.UnsafeReset(ctx); err != nil {
				t.Fatal(err)
			}
		}()

		for i := 0; i < len(rows); i++ {
			if err := rc.AddRow(ctx, rows[i]); err != nil {
				t.Fatal(err)
			}
		}
		if rc.UsingDisk() {
			t.Fatal("unexpectedly using disk")
		}
		func() {
			// We haven't marked any rows, so the unmarked iterator should iterate
			// over all rows added so far.
			i := rc.NewUnmarkedIterator(ctx)
			defer i.Close()
			if err := verifyRows(ctx, i, rows, &evalCtx, ordering); err != nil {
				t.Fatalf("verifying memory rows failed with: %s", err)
			}
		}()
		if err := rc.reserveMarkMemoryMaybe(ctx); err != nil {
			t.Fatal(err)
		}
		func() {
			i, err := rc.NewBucketIterator(ctx, rows[0], storedEqColumns)
			if err != nil {
				t.Fatal(err)
			}
			defer i.Close()
			for i.Rewind(); ; i.Next() {
				if ok, err := i.Valid(); err != nil {
					t.Fatal(err)
				} else if !ok {
					break
				}
				if err := i.Mark(ctx, true); err != nil {
					t.Fatal(err)
				}
			}
		}()
		if err := rc.spillToDisk(ctx); err != nil {
			t.Fatal(err)
		}
		if !rc.UsingDisk() {
			t.Fatal("unexpectedly using memory")
		}
		func() {
			i, err := rc.NewBucketIterator(ctx, rows[0], storedEqColumns)
			if err != nil {
				t.Fatal(err)
			}
			defer i.Close()
			for i.Rewind(); ; i.Next() {
				if ok, err := i.Valid(); err != nil {
					t.Fatal(err)
				} else if !ok {
					break
				}
				if !i.IsMarked(ctx) {
					t.Fatal("Mark is not preserved during spilling to disk")
				}
			}
		}()
	})
}