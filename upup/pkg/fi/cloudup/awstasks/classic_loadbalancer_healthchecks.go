/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package awstasks

import (
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancing/types"
	"k8s.io/kops/upup/pkg/fi"
)

type ClassicLoadBalancerHealthCheck struct {
	Target *string

	HealthyThreshold   *int32
	UnhealthyThreshold *int32

	Interval *int32
	Timeout  *int32
}

var _ fi.CloudupHasDependencies = &ClassicLoadBalancerListener{}

func (e *ClassicLoadBalancerHealthCheck) GetDependencies(tasks map[string]fi.CloudupTask) []fi.CloudupTask {
	return nil
}

func findHealthCheck(lb *elbtypes.LoadBalancerDescription) (*ClassicLoadBalancerHealthCheck, error) {
	if lb == nil || lb.HealthCheck == nil {
		return nil, nil
	}

	actual := &ClassicLoadBalancerHealthCheck{}
	if lb.HealthCheck != nil {
		actual.Target = lb.HealthCheck.Target
		actual.HealthyThreshold = lb.HealthCheck.HealthyThreshold
		actual.UnhealthyThreshold = lb.HealthCheck.UnhealthyThreshold
		actual.Interval = lb.HealthCheck.Interval
		actual.Timeout = lb.HealthCheck.Timeout
	}

	return actual, nil
}
