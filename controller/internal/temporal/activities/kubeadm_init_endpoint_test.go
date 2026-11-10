// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package activities

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dsx-ai-factory/nvke/controller/internal/db"
)

func TestResolveKubeadmControlPlaneEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		cluster      db.Cluster
		wantEndpoint string
		wantSANs     []string
		wantErr      bool
	}{
		{name: "no stable endpoint uses the node IP", cluster: db.Cluster{}, wantEndpoint: "10.1.1.1"},
		{name: "kube-vip uses the VIP without a port", cluster: db.Cluster{ControlPlaneVIP: "10.0.0.100"}, wantEndpoint: "10.0.0.100"},
		{
			name: "external uses host and port and adds the host to the certificate",
			cluster: db.Cluster{
				ControlPlaneEndpointType: db.ControlPlaneEndpointExternal,
				ControlPlaneEndpointHost: "api.tenant-a.example.internal",
				ControlPlaneEndpointPort: 6443,
			},
			wantEndpoint: "api.tenant-a.example.internal:6443",
			wantSANs:     []string{"api.tenant-a.example.internal"},
		},
		{
			name:    "infrastructure with no reported host fails",
			cluster: db.Cluster{ControlPlaneEndpointType: db.ControlPlaneEndpointInfrastructure},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, sans, err := resolveKubeadmControlPlaneEndpoint(tt.cluster, "10.1.1.1")
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantEndpoint, endpoint)
			assert.Equal(t, tt.wantSANs, sans)
		})
	}
}
