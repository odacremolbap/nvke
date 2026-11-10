// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package db

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClusterControlPlaneEndpoint(t *testing.T) {
	tests := []struct {
		name         string
		cluster      Cluster
		wantType     ControlPlaneEndpointType
		wantStable   bool
		wantKubeVIP  bool
		wantLB       bool
		wantHostPort string
	}{
		{name: "row with neither", cluster: Cluster{}, wantType: ControlPlaneEndpointNone, wantHostPort: ":6443"},
		{
			name:         "row that predates the type column has a VIP",
			cluster:      Cluster{ControlPlaneVIP: "10.0.0.100"},
			wantType:     ControlPlaneEndpointKubeVIP,
			wantStable:   true,
			wantKubeVIP:  true,
			wantHostPort: "10.0.0.100:6443",
		},
		{
			name:         "external with a port",
			cluster:      Cluster{ControlPlaneEndpointType: ControlPlaneEndpointExternal, ControlPlaneEndpointHost: "lb.example.internal", ControlPlaneEndpointPort: 8443},
			wantType:     ControlPlaneEndpointExternal,
			wantStable:   true,
			wantLB:       true,
			wantHostPort: "lb.example.internal:8443",
		},
		{
			name:         "external without a port uses 6443",
			cluster:      Cluster{ControlPlaneEndpointType: ControlPlaneEndpointExternal, ControlPlaneEndpointHost: "lb.example.internal"},
			wantType:     ControlPlaneEndpointExternal,
			wantStable:   true,
			wantLB:       true,
			wantHostPort: "lb.example.internal:6443",
		},
		{
			name:         "infrastructure before the provider reports a host",
			cluster:      Cluster{ControlPlaneEndpointType: ControlPlaneEndpointInfrastructure},
			wantType:     ControlPlaneEndpointInfrastructure,
			wantStable:   true,
			wantLB:       true,
			wantHostPort: ":6443",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantType, tt.cluster.EndpointType())
			assert.Equal(t, tt.wantStable, tt.cluster.HasStableControlPlaneEndpoint())
			assert.Equal(t, tt.wantKubeVIP, tt.cluster.UsesKubeVIP())
			assert.Equal(t, tt.wantLB, tt.cluster.IsLoadBalancedEndpoint())
			host, port := tt.cluster.ControlPlaneEndpointAddress()
			assert.Equal(t, tt.wantHostPort, net.JoinHostPort(host, strconv.Itoa(port)))
		})
	}
}
