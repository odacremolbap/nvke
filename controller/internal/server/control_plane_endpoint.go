// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package server

import (
	"fmt"
	"net/netip"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"

	api "github.com/dsx-ai-factory/nvke/controller/api"
	"github.com/dsx-ai-factory/nvke/controller/internal/db"
)

// requestHasControlPlaneEndpoint reports whether a create request names a stable API
// server endpoint, through either the legacy controlPlaneVIP or controlPlaneEndpoint.
func requestHasControlPlaneEndpoint(req *api.CreateClusterRequest) bool {
	return strings.TrimSpace(req.ControlPlaneVIP) != "" || req.ControlPlaneEndpoint != nil
}

// validateControlPlaneEndpoint validates controlPlaneVIP and controlPlaneEndpoint together.
func validateControlPlaneEndpoint(req *api.CreateClusterRequest, network resolvedClusterNetwork) error {
	vip := strings.TrimSpace(req.ControlPlaneVIP)
	endpoint := req.ControlPlaneEndpoint
	if vip != "" && endpoint != nil {
		return newBadRequestErrorf("controlPlaneVIP and controlPlaneEndpoint are mutually exclusive; use controlPlaneEndpoint with type KubeVIP")
	}
	if vip != "" {
		if err := network.validateControlPlaneVIP(vip); err != nil {
			return newBadRequestErrorf("%s", err)
		}
		return nil
	}
	if endpoint == nil {
		return nil
	}

	switch endpoint.Type {
	case api.ControlPlaneEndpointTypeKubeVIP:
		if endpoint.KubeVIP == nil {
			return newBadRequestErrorf("controlPlaneEndpoint.kubeVIP is required when type is %s", endpoint.Type)
		}
		if endpoint.External != nil {
			return newBadRequestErrorf("controlPlaneEndpoint.external is not allowed when type is %s", endpoint.Type)
		}
		if err := network.validateControlPlaneVIP(strings.TrimSpace(endpoint.KubeVIP.Address)); err != nil {
			return newBadRequestErrorf("%s", err)
		}
	case api.ControlPlaneEndpointTypeExternal:
		if endpoint.External == nil {
			return newBadRequestErrorf("controlPlaneEndpoint.external is required when type is %s", endpoint.Type)
		}
		if endpoint.KubeVIP != nil {
			return newBadRequestErrorf("controlPlaneEndpoint.kubeVIP is not allowed when type is %s", endpoint.Type)
		}
		if err := validateExternalEndpoint(*endpoint.External); err != nil {
			return newBadRequestErrorf("%s", err)
		}
	case api.ControlPlaneEndpointTypeInfrastructure:
		if endpoint.KubeVIP != nil || endpoint.External != nil {
			return newBadRequestErrorf("controlPlaneEndpoint.kubeVIP and controlPlaneEndpoint.external are not allowed when type is %s", endpoint.Type)
		}
		if req.Infra == nil {
			return newBadRequestErrorf("controlPlaneEndpoint type %s requires a cluster on the generic CAPI provider (infra)", endpoint.Type)
		}
	default:
		return newBadRequestErrorf("invalid controlPlaneEndpoint.type %q", endpoint.Type)
	}
	return nil
}

// validateExternalEndpoint accepts a DNS name or an IPv4 address, and a TCP port.
func validateExternalEndpoint(external api.ExternalEndpoint) error {
	host := strings.TrimSpace(external.Host)
	if host == "" {
		return fmt.Errorf("controlPlaneEndpoint.external.host must not be empty")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if !addr.Is4() {
			return fmt.Errorf("invalid controlPlaneEndpoint.external.host %q: IPv6 is not supported, use an IPv4 address or a DNS name", host)
		}
	} else if errs := validation.IsDNS1123Subdomain(host); len(errs) > 0 {
		return fmt.Errorf("invalid controlPlaneEndpoint.external.host %q: must be a DNS name or an IPv4 address: %s", host, strings.Join(errs, "; "))
	}
	if external.Port != 0 && (external.Port < 1 || external.Port > 65535) {
		return fmt.Errorf("invalid controlPlaneEndpoint.external.port %d: must be between 1 and 65535", external.Port)
	}
	return nil
}

// applyControlPlaneEndpoint sets the cluster's endpoint columns from a validated create
// request. The legacy controlPlaneVIP and type KubeVIP are stored the same way: in
// control_plane_vip with no type, so an older controller reads them unchanged.
func applyControlPlaneEndpoint(cluster *db.Cluster, req *api.CreateClusterRequest) {
	if req.ControlPlaneEndpoint == nil {
		return
	}
	endpoint := req.ControlPlaneEndpoint
	switch endpoint.Type {
	case api.ControlPlaneEndpointTypeKubeVIP:
		cluster.ControlPlaneVIP = strings.TrimSpace(endpoint.KubeVIP.Address)
	case api.ControlPlaneEndpointTypeExternal:
		cluster.ControlPlaneEndpointType = db.ControlPlaneEndpointExternal
		cluster.ControlPlaneEndpointHost = strings.TrimSpace(endpoint.External.Host)
		cluster.ControlPlaneEndpointPort = endpoint.External.Port
		if cluster.ControlPlaneEndpointPort == 0 {
			cluster.ControlPlaneEndpointPort = db.DefaultControlPlaneEndpointPort
		}
	case api.ControlPlaneEndpointTypeInfrastructure:
		cluster.ControlPlaneEndpointType = db.ControlPlaneEndpointInfrastructure
	}
}

// apiControlPlaneEndpointFromDB reports a cluster's endpoint in the API form when it is
// a load balancer. A kube-vip cluster keeps reporting controlPlaneVIP only, as it
// always has, and a cluster with no stable endpoint reports neither.
func apiControlPlaneEndpointFromDB(cluster *db.Cluster) *api.ControlPlaneEndpoint {
	switch cluster.EndpointType() {
	case db.ControlPlaneEndpointExternal:
		host, port := cluster.ControlPlaneEndpointAddress()
		return &api.ControlPlaneEndpoint{
			Type:     api.ControlPlaneEndpointTypeExternal,
			External: &api.ExternalEndpoint{Host: host, Port: port},
		}
	case db.ControlPlaneEndpointInfrastructure:
		return &api.ControlPlaneEndpoint{Type: api.ControlPlaneEndpointTypeInfrastructure}
	case db.ControlPlaneEndpointKubeVIP, db.ControlPlaneEndpointNone:
		return nil
	}
	return nil
}
