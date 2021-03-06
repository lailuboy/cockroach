// Copyright 2016 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License included
// in the file licenses/BSL.txt and at www.mariadb.com/bsl11.
//
// Change Date: 2022-10-01
//
// On the date above, in accordance with the Business Source License, use
// of this software will be governed by the Apache License, Version 2.0,
// included in the file licenses/APL.txt and at
// https://www.apache.org/licenses/LICENSE-2.0

package sql

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlbase"
)

// delayedNode wraps a planNode in cases where the planNode
// constructor must be delayed during query execution (as opposed to
// SQL prepare) for resource tracking purposes.
type delayedNode struct {
	name        string
	columns     sqlbase.ResultColumns
	constructor nodeConstructor
	plan        planNode
}

// delayedNode implements the autoCommitNode interface.
var _ autoCommitNode = &delayedNode{}

type nodeConstructor func(context.Context, *planner) (planNode, error)

func (d *delayedNode) Next(params runParams) (bool, error) { return d.plan.Next(params) }
func (d *delayedNode) Values() tree.Datums                 { return d.plan.Values() }

func (d *delayedNode) Close(ctx context.Context) {
	if d.plan != nil {
		d.plan.Close(ctx)
		d.plan = nil
	}
}

// enableAutoCommit is part of the autoCommitNode interface.
func (d *delayedNode) enableAutoCommit() {
	if ac, ok := d.plan.(autoCommitNode); ok {
		ac.enableAutoCommit()
	}
}

// startExec constructs the wrapped planNode now that execution is underway.
func (d *delayedNode) startExec(params runParams) error {
	if d.plan != nil {
		panic("wrapped plan should not yet exist")
	}

	plan, err := d.constructor(params.ctx, params.p)
	if err != nil {
		return err
	}
	d.plan = plan

	// Recursively invoke startExec on new plan. Normally, startExec doesn't
	// recurse - calling children is handled by the planNode walker. The reason
	// this won't suffice here is that the the child of this node doesn't exist
	// until after startExec is invoked.
	return startExec(params, plan)
}
