// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	api "github.com/dsx-ai-factory/nvke/controller/api"
	"github.com/dsx-ai-factory/nvke/controller/internal/db"
)

func TestValidateControlPlaneEndpoint(t *testing.T) {
	network, err := resolveClusterNetwork(nil)
	require.NoError(t, err)

	external := func(host string, port int) *api.ControlPlaneEndpoint {
		return &api.ControlPlaneEndpoint{
			Type:     api.ControlPlaneEndpointTypeExternal,
			External: &api.ExternalEndpoint{Host: host, Port: port},
		}
	}

	tests := []struct {
		name    string
		req     api.CreateClusterRequest
		wantErr string
	}{
		{name: "nothing set"},
		{name: "legacy controlPlaneVIP", req: api.CreateClusterRequest{ControlPlaneVIP: "10.0.0.100"}},
		{
			name:    "legacy VIP inside the pod CIDR",
			req:     api.CreateClusterRequest{ControlPlaneVIP: "192.168.1.1"},
			wantErr: "overlaps",
		},
		{
			name:    "VIP and endpoint together",
			req:     api.CreateClusterRequest{ControlPlaneVIP: "10.0.0.100", ControlPlaneEndpoint: external("lb.example.internal", 0)},
			wantErr: "mutually exclusive",
		},
		{
			name: "KubeVIP",
			req: api.CreateClusterRequest{ControlPlaneEndpoint: &api.ControlPlaneEndpoint{
				Type: api.ControlPlaneEndpointTypeKubeVIP, KubeVIP: &api.KubeVIPEndpoint{Address: "10.0.0.100"},
			}},
		},
		{
			name:    "KubeVIP without its address",
			req:     api.CreateClusterRequest{ControlPlaneEndpoint: &api.ControlPlaneEndpoint{Type: api.ControlPlaneEndpointTypeKubeVIP}},
			wantErr: "kubeVIP is required",
		},
		{
			name: "KubeVIP with external",
			req: api.CreateClusterRequest{ControlPlaneEndpoint: &api.ControlPlaneEndpoint{
				Type:     api.ControlPlaneEndpointTypeKubeVIP,
				KubeVIP:  &api.KubeVIPEndpoint{Address: "10.0.0.100"},
				External: &api.ExternalEndpoint{Host: "lb.example.internal"},
			}},
			wantErr: "external is not allowed",
		},
		{name: "External DNS name", req: api.CreateClusterRequest{ControlPlaneEndpoint: external("api.tenant-a.example.internal", 0)}},
		{name: "External IPv4 with a port", req: api.CreateClusterRequest{ControlPlaneEndpoint: external("10.0.0.50", 8443)}},
		{
			name:    "External without its host",
			req:     api.CreateClusterRequest{ControlPlaneEndpoint: &api.ControlPlaneEndpoint{Type: api.ControlPlaneEndpointTypeExternal}},
			wantErr: "external is required",
		},
		{name: "External empty host", req: api.CreateClusterRequest{ControlPlaneEndpoint: external(" ", 0)}, wantErr: "must not be empty"},
		{name: "External IPv6", req: api.CreateClusterRequest{ControlPlaneEndpoint: external("2001:db8::1", 0)}, wantErr: "IPv6 is not supported"},
		{name: "External host with a scheme", req: api.CreateClusterRequest{ControlPlaneEndpoint: external("https://lb.example", 0)}, wantErr: "DNS name or an IPv4"},
		{name: "External port too large", req: api.CreateClusterRequest{ControlPlaneEndpoint: external("lb.example.internal", 70000)}, wantErr: "between 1 and 65535"},
		{
			name: "Infrastructure on the generic CAPI provider",
			req: api.CreateClusterRequest{
				Infra:                &api.InfraClusterProvider{},
				ControlPlaneEndpoint: &api.ControlPlaneEndpoint{Type: api.ControlPlaneEndpointTypeInfrastructure},
			},
		},
		{
			name:    "Infrastructure on another provider",
			req:     api.CreateClusterRequest{ControlPlaneEndpoint: &api.ControlPlaneEndpoint{Type: api.ControlPlaneEndpointTypeInfrastructure}},
			wantErr: "generic CAPI provider",
		},
		{
			name:    "unknown type",
			req:     api.CreateClusterRequest{ControlPlaneEndpoint: &api.ControlPlaneEndpoint{Type: "Other"}},
			wantErr: "invalid controlPlaneEndpoint.type",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateControlPlaneEndpoint(&tt.req, network)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestApplyControlPlaneEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint *api.ControlPlaneEndpoint
		want     db.Cluster
	}{
		{name: "none", want: db.Cluster{}},
		{
			name: "KubeVIP is stored as the legacy VIP",
			endpoint: &api.ControlPlaneEndpoint{
				Type: api.ControlPlaneEndpointTypeKubeVIP, KubeVIP: &api.KubeVIPEndpoint{Address: " 10.0.0.100 "},
			},
			want: db.Cluster{ControlPlaneVIP: "10.0.0.100"},
		},
		{
			name: "External defaults the port",
			endpoint: &api.ControlPlaneEndpoint{
				Type: api.ControlPlaneEndpointTypeExternal, External: &api.ExternalEndpoint{Host: "lb.example.internal"},
			},
			want: db.Cluster{
				ControlPlaneEndpointType: db.ControlPlaneEndpointExternal,
				ControlPlaneEndpointHost: "lb.example.internal",
				ControlPlaneEndpointPort: 6443,
			},
		},
		{
			name:     "Infrastructure has no host until the provider reports one",
			endpoint: &api.ControlPlaneEndpoint{Type: api.ControlPlaneEndpointTypeInfrastructure},
			want:     db.Cluster{ControlPlaneEndpointType: db.ControlPlaneEndpointInfrastructure},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got db.Cluster
			applyControlPlaneEndpoint(&got, &api.CreateClusterRequest{ControlPlaneEndpoint: tt.endpoint})
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAPIControlPlaneEndpointFromDB(t *testing.T) {
	assert.Nil(t, apiControlPlaneEndpointFromDB(&db.Cluster{}))
	assert.Nil(t, apiControlPlaneEndpointFromDB(&db.Cluster{ControlPlaneVIP: "10.0.0.100"}), "kube-vip clusters report controlPlaneVIP only")

	got := apiControlPlaneEndpointFromDB(&db.Cluster{
		ControlPlaneEndpointType: db.ControlPlaneEndpointExternal,
		ControlPlaneEndpointHost: "lb.example.internal",
		ControlPlaneEndpointPort: 8443,
	})
	require.NotNil(t, got)
	assert.Equal(t, api.ControlPlaneEndpointTypeExternal, got.Type)
	assert.Equal(t, api.ExternalEndpoint{Host: "lb.example.internal", Port: 8443}, *got.External)
}

func TestControlPlaneGatesAcceptAnyStableEndpoint(t *testing.T) {
	s := &clusterService{}
	external := db.Cluster{ControlPlaneEndpointType: db.ControlPlaneEndpointExternal, ControlPlaneEndpointHost: "lb.example.internal"}
	vip := db.Cluster{ControlPlaneVIP: "10.0.0.100"}
	none := db.Cluster{}

	for name, cluster := range map[string]db.Cluster{"external": external, "vip": vip} {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, s.validateControlPlaneCapacityChange(cluster.HasStableControlPlaneEndpoint(), 1, 3))
			require.NoError(t, s.validateControlPlaneUpgrade(cluster.HasStableControlPlaneEndpoint()))
			require.NoError(t, s.validateControlPlaneRollout(cluster.HasStableControlPlaneEndpoint(), true))
		})
	}
	t.Run("none", func(t *testing.T) {
		require.Error(t, s.validateControlPlaneCapacityChange(none.HasStableControlPlaneEndpoint(), 1, 3))
		require.Error(t, s.validateControlPlaneUpgrade(none.HasStableControlPlaneEndpoint()))
		require.Error(t, s.validateControlPlaneRollout(none.HasStableControlPlaneEndpoint(), true))
	})
}
