// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// clusters.control_plane_endpoint_type, control_plane_endpoint_host and
// control_plane_endpoint_port describe how a cluster's API server is reached when it
// is not through kube-vip. All three are nullable and nothing is backfilled: a row
// with no type and a control_plane_vip is read as a kube-vip cluster, and a row with
// neither is a cluster without a stable endpoint. Rolling upgrades leave old
// controller pods writing rows with no type, so a backfill would not stay true.
//
//nolint:gochecknoinits // Bun registers migrations at package initialization.
func init() {
	migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		if _, err := db.ExecContext(ctx, `
			ALTER TABLE clusters
				ADD COLUMN IF NOT EXISTS control_plane_endpoint_type text,
				ADD COLUMN IF NOT EXISTS control_plane_endpoint_host text,
				ADD COLUMN IF NOT EXISTS control_plane_endpoint_port integer
		`); err != nil {
			return fmt.Errorf("add clusters.control_plane_endpoint columns: %w", err)
		}
		return nil
	}, func(context.Context, *bun.DB) error {
		return fmt.Errorf("down migration not supported")
	})
}
