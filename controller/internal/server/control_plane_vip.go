// SPDX-FileCopyrightText: Copyright (c) 2026 NVIDIA CORPORATION & AFFILIATES. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package server

const (
	controlPlaneVIPRequiredMessage = "a control-plane endpoint (controlPlaneVIP or controlPlaneEndpoint) is required to increase desired control-plane capacity above one; recreate the cluster with one configured"
	controlPlaneVIPUpgradeMessage  = "a control-plane endpoint (controlPlaneVIP or controlPlaneEndpoint) is required to upgrade the control plane; recreate the cluster with one configured"
	controlPlaneVIPRolloutMessage  = "a control-plane endpoint (controlPlaneVIP or controlPlaneEndpoint) is required to replace control-plane nodes; recreate the cluster with one configured before changing rollout-triggering control-plane configuration"
)

// validateControlPlaneCapacityChange lets existing clusters without a stable endpoint keep or reduce
// capacity, but rejects growth beyond one control-plane node unless the dev override is enabled.
func (s *clusterService) validateControlPlaneCapacityChange(hasStableEndpoint bool, current, desired int) error {
	if !s.allowControlPlaneWithoutVIP && !hasStableEndpoint && desired > 1 && desired > current {
		return newBadRequestErrorf("%s", controlPlaneVIPRequiredMessage)
	}
	return nil
}

func (s *clusterService) validateControlPlaneUpgrade(hasStableEndpoint bool) error {
	if !s.allowControlPlaneWithoutVIP && !hasStableEndpoint {
		return newBadRequestErrorf("%s", controlPlaneVIPUpgradeMessage)
	}
	return nil
}

func (s *clusterService) validateControlPlaneRollout(hasStableEndpoint, rolloutRequired bool) error {
	if !s.allowControlPlaneWithoutVIP && !hasStableEndpoint && rolloutRequired {
		return newBadRequestErrorf("%s", controlPlaneVIPRolloutMessage)
	}
	return nil
}
