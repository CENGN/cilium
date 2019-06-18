// Copyright 2019 Authors of Cilium
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build !privileged_tests

package eni

import (
	"fmt"
	"time"

	ec2mock "github.com/cilium/cilium/pkg/aws/ec2/mock"
	metricsmock "github.com/cilium/cilium/pkg/aws/eni/metrics/mock"
	"github.com/cilium/cilium/pkg/aws/types"
	"github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/testutils"

	"gopkg.in/check.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	testSubnet = &types.Subnet{
		ID:                 "s-1",
		AvailabilityZone:   "us-west-1",
		VpcID:              "vpc-1",
		AvailableAddresses: 100,
		Tags:               types.Tags{"k": "v"},
	}
	k8sapi     = &k8sMock{}
	metricsapi = metricsmock.NewMockMetrics()
)

func (e *ENISuite) TestGetNodeNames(c *check.C) {
	ec2api := ec2mock.NewAPI([]*types.Subnet{testSubnet})
	mngr, err := NewNodeManager(ec2api, ec2api, k8sapi, metricsapi)
	c.Assert(err, check.IsNil)
	c.Assert(mngr, check.Not(check.IsNil))

	mngr.Update(newCiliumNode("node1", "i-1", "m4.large", "us-west-1", "vpc-1", 0, 0, 0, 0))

	names := mngr.GetNames()
	c.Assert(len(names), check.Equals, 1)
	c.Assert(names[0], check.Equals, "node1")

	mngr.Update(newCiliumNode("node2", "i-2", "m4.large", "us-west-1", "vpc-1", 0, 0, 0, 0))

	names = mngr.GetNames()
	c.Assert(len(names), check.Equals, 2)

	mngr.Delete("node1")

	names = mngr.GetNames()
	c.Assert(len(names), check.Equals, 1)
	c.Assert(names[0], check.Equals, "node2")
}

func (e *ENISuite) TestNodeManagerGet(c *check.C) {
	ec2api := ec2mock.NewAPI([]*types.Subnet{testSubnet})
	mngr, err := NewNodeManager(ec2api, ec2api, k8sapi, metricsapi)
	c.Assert(err, check.IsNil)
	c.Assert(mngr, check.Not(check.IsNil))

	mngr.Update(newCiliumNode("node1", "i-1", "m4.large", "us-west-1", "vpc-1", 0, 0, 0, 0))

	c.Assert(mngr.Get("node1"), check.Not(check.IsNil))
	c.Assert(mngr.Get("node2"), check.IsNil)

	mngr.Delete("node1")
	c.Assert(mngr.Get("node1"), check.IsNil)
	c.Assert(mngr.Get("node2"), check.IsNil)
}

type k8sMock struct{}

func (k *k8sMock) Update(node, origNode *v2.CiliumNode) (*v2.CiliumNode, error) {
	return nil, nil
}

func (k *k8sMock) UpdateStatus(node, origNode *v2.CiliumNode) (*v2.CiliumNode, error) {
	return nil, nil
}

func newCiliumNode(node, instanceID, instanceType, az, vpcID string, preAllocate, minAllocate, available, used int) *v2.CiliumNode {
	cn := &v2.CiliumNode{
		ObjectMeta: metav1.ObjectMeta{Name: node, Namespace: "default"},
		Spec: v2.NodeSpec{
			ENI: v2.ENISpec{
				InstanceID:       instanceID,
				InstanceType:     instanceType,
				PreAllocate:      preAllocate,
				MinAllocate:      minAllocate,
				AvailabilityZone: az,
				VpcID:            vpcID,
			},
			IPAM: v2.IPAMSpec{
				Pool: map[string]v2.AllocationIP{},
			},
		},
		Status: v2.NodeStatus{
			IPAM: v2.IPAMStatus{
				Used: map[string]v2.AllocationIP{},
			},
		},
	}

	updateCiliumNode(cn, available, used)

	return cn
}

func updateCiliumNode(cn *v2.CiliumNode, available, used int) *v2.CiliumNode {
	cn.Spec.IPAM.Pool = map[string]v2.AllocationIP{}
	for i := 0; i < used; i++ {
		cn.Spec.IPAM.Pool[fmt.Sprintf("1.1.1.%d", i)] = v2.AllocationIP{Resource: "foo"}
	}

	cn.Status.IPAM.Used = map[string]v2.AllocationIP{}
	for ip, ipAllocation := range cn.Spec.IPAM.Pool {
		if used > 0 {
			delete(cn.Spec.IPAM.Pool, ip)
			cn.Status.IPAM.Used[ip] = ipAllocation
			used--
		}
	}

	return cn
}

func reachedAddressesNeeded(mngr *NodeManager, nodeName string, needed int) (success bool) {
	if node := mngr.Get(nodeName); node != nil {
		success = node.getNeededAddresses() == needed
	}
	return
}

// TestNodeManagerDefaultAllocation tests allocation with default parameters
//
// - m4.large (2x ENIs, 2x10 IPs)
// - MinAllocate 0
// - PreAllocate 0 (default: 8)
func (e *ENISuite) TestNodeManagerDefaultAllocation(c *check.C) {
	ec2api := ec2mock.NewAPI([]*types.Subnet{testSubnet})
	mngr, err := NewNodeManager(ec2api, ec2api, k8sapi, metricsapi)
	c.Assert(err, check.IsNil)
	c.Assert(mngr, check.Not(check.IsNil))

	// Announce node wait for IPs to become available
	cn := newCiliumNode("node1", "i-0", "m4.large", "us-west-1", "vpc-1", 0, 0, 0, 0)
	mngr.Update(cn)
	c.Assert(testutils.WaitUntil(func() bool { return reachedAddressesNeeded(mngr, "node1", 0) }, 5*time.Second), check.IsNil)

	node := mngr.Get("node1")
	c.Assert(node, check.Not(check.IsNil))
	c.Assert(node.stats.availableIPs, check.Equals, 8)
	c.Assert(node.stats.usedIPs, check.Equals, 0)

	// Use 7 out of 8 IPs
	mngr.Update(updateCiliumNode(cn, 8, 7))
	c.Assert(testutils.WaitUntil(func() bool { return reachedAddressesNeeded(mngr, "node1", 0) }, 5*time.Second), check.IsNil)

	node = mngr.Get("node1")
	c.Assert(node, check.Not(check.IsNil))
	c.Assert(node.stats.availableIPs, check.Equals, 15)
	c.Assert(node.stats.usedIPs, check.Equals, 7)
}

// TestNodeManagerMinAllocate20 tests MinAllocate without PreAllocate
//
// - m4.large (2x ENIs, 2x10 IPs)
// - MinAllocate 10
// - PreAllocate -1
func (e *ENISuite) TestNodeManagerMinAllocate20(c *check.C) {
	ec2api := ec2mock.NewAPI([]*types.Subnet{testSubnet})
	mngr, err := NewNodeManager(ec2api, ec2api, k8sapi, metricsapi)
	c.Assert(err, check.IsNil)
	c.Assert(mngr, check.Not(check.IsNil))

	// Announce node wait for IPs to become available
	cn := newCiliumNode("node2", "i-1", "m5.4xlarge", "us-west-1", "vpc-1", -1, 10, 0, 0)
	mngr.Update(cn)
	c.Assert(testutils.WaitUntil(func() bool { return reachedAddressesNeeded(mngr, "node2", 0) }, 5*time.Second), check.IsNil)

	node := mngr.Get("node2")
	c.Assert(node, check.Not(check.IsNil))
	c.Assert(node.stats.availableIPs, check.Equals, 10)
	c.Assert(node.stats.usedIPs, check.Equals, 0)

	mngr.Update(updateCiliumNode(cn, 10, 8))
	c.Assert(testutils.WaitUntil(func() bool { return reachedAddressesNeeded(mngr, "node2", 0) }, 5*time.Second), check.IsNil)

	node = mngr.Get("node2")
	c.Assert(node, check.Not(check.IsNil))
	c.Assert(node.stats.availableIPs, check.Equals, 10)
	c.Assert(node.stats.usedIPs, check.Equals, 8)

	// Change MinAllocate to 20
	cn = newCiliumNode("node2", "i-1", "m5.4xlarge", "us-west-1", "vpc-1", 0, 20, 10, 8)
	mngr.Update(cn)
	c.Assert(testutils.WaitUntil(func() bool { return reachedAddressesNeeded(mngr, "node2", 0) }, 5*time.Second), check.IsNil)

	node = mngr.Get("node2")
	c.Assert(node, check.Not(check.IsNil))
	c.Assert(node.stats.availableIPs, check.Equals, 20)
	c.Assert(node.stats.usedIPs, check.Equals, 8)
}

// TestNodeManagerMinAllocateAndPreallocate tests MinAllocate in combination with PreAllocate
//
// - m4.large (2x ENIs, 2x10 IPs)
// - MinAllocate 10
// - PreAllocate 1
func (e *ENISuite) TestNodeManagerMinAllocateAndPreallocate(c *check.C) {
	ec2api := ec2mock.NewAPI([]*types.Subnet{testSubnet})
	mngr, err := NewNodeManager(ec2api, ec2api, k8sapi, metricsapi)
	c.Assert(err, check.IsNil)
	c.Assert(mngr, check.Not(check.IsNil))

	// Announce node, wait for IPs to become available
	cn := newCiliumNode("node2", "i-1", "m4.large", "us-west-1", "vpc-1", 1, 10, 0, 0)
	mngr.Update(cn)
	c.Assert(testutils.WaitUntil(func() bool { return reachedAddressesNeeded(mngr, "node2", 0) }, 5*time.Second), check.IsNil)

	node := mngr.Get("node2")
	c.Assert(node, check.Not(check.IsNil))
	c.Assert(node.stats.availableIPs, check.Equals, 10)
	c.Assert(node.stats.usedIPs, check.Equals, 0)

	// Use 9 out of 10 IPs, no additional IPs should be allocated
	mngr.Update(updateCiliumNode(cn, 10, 9))
	c.Assert(testutils.WaitUntil(func() bool { return reachedAddressesNeeded(mngr, "node2", 0) }, 5*time.Second), check.IsNil)
	node = mngr.Get("node2")
	c.Assert(node, check.Not(check.IsNil))
	c.Assert(node.stats.availableIPs, check.Equals, 10)
	c.Assert(node.stats.usedIPs, check.Equals, 9)

	// Use 10 out of 10 IPs, PreAllocate 1 must kick in and allocate an additional IP
	mngr.Update(updateCiliumNode(cn, 10, 10))
	c.Assert(testutils.WaitUntil(func() bool { return reachedAddressesNeeded(mngr, "node2", 0) }, 5*time.Second), check.IsNil)
	node = mngr.Get("node2")
	c.Assert(node, check.Not(check.IsNil))
	c.Assert(node.stats.availableIPs, check.Equals, 11)
	c.Assert(node.stats.usedIPs, check.Equals, 10)

	// Release some IPs, no additional IPs should be allocated
	mngr.Update(updateCiliumNode(cn, 10, 8))
	c.Assert(testutils.WaitUntil(func() bool { return reachedAddressesNeeded(mngr, "node2", 0) }, 5*time.Second), check.IsNil)
	node = mngr.Get("node2")
	c.Assert(node, check.Not(check.IsNil))
	c.Assert(node.stats.availableIPs, check.Equals, 11)
	c.Assert(node.stats.usedIPs, check.Equals, 8)
}

// TestNodeManagerExceedENICapacity tests exceeding ENI capacity
//
// - m4.large (2x ENIs, 2x10 IPs)
// - MinAllocate 20
// - PreAllocate 8
func (e *ENISuite) TestNodeManagerExceedENICapacity(c *check.C) {
	ec2api := ec2mock.NewAPI([]*types.Subnet{testSubnet})
	mngr, err := NewNodeManager(ec2api, ec2api, k8sapi, metricsapi)
	c.Assert(err, check.IsNil)
	c.Assert(mngr, check.Not(check.IsNil))

	// Announce node, wait for IPs to become available
	cn := newCiliumNode("node2", "i-1", "m4.large", "us-west-1", "vpc-1", 8, 20, 0, 0)
	mngr.Update(cn)
	c.Assert(testutils.WaitUntil(func() bool { return reachedAddressesNeeded(mngr, "node2", 0) }, 5*time.Second), check.IsNil)

	node := mngr.Get("node2")
	c.Assert(node, check.Not(check.IsNil))
	c.Assert(node.stats.availableIPs, check.Equals, 20)
	c.Assert(node.stats.usedIPs, check.Equals, 0)

	// Use 16 out of 20 IPs, we should reach 4 addresses needed but never 0 addresses needed
	mngr.Update(updateCiliumNode(cn, 20, 16))
	c.Assert(testutils.WaitUntil(func() bool { return reachedAddressesNeeded(mngr, "node2", 4) }, 5*time.Second), check.IsNil)
	c.Assert(testutils.WaitUntil(func() bool { return reachedAddressesNeeded(mngr, "node2", 0) }, 5*time.Second), check.Not(check.IsNil))

	node = mngr.Get("node2")
	c.Assert(node, check.Not(check.IsNil))
	c.Assert(node.stats.availableIPs, check.Equals, 20)
	c.Assert(node.stats.usedIPs, check.Equals, 16)
}

// TestNodeManagerManyNodes tests IP allocation of 100 nodes across 3 subnets
//
// - m4.large (2x ENIs, 2x10 IPs)
// - MinAllocate 10
// - PreAllocate 1
func (e *ENISuite) TestNodeManagerManyNodes(c *check.C) {
	testSubnet1 := &types.Subnet{ID: "s-1", AvailabilityZone: "us-west-1", VpcID: "vpc-1", AvailableAddresses: 400}
	testSubnet2 := &types.Subnet{ID: "s-2", AvailabilityZone: "us-west-1", VpcID: "vpc-1", AvailableAddresses: 400}
	testSubnet3 := &types.Subnet{ID: "s-3", AvailabilityZone: "us-west-1", VpcID: "vpc-1", AvailableAddresses: 400}

	ec2api := ec2mock.NewAPI([]*types.Subnet{testSubnet1, testSubnet2, testSubnet3})
	mngr, err := NewNodeManager(ec2api, ec2api, k8sapi, metricsapi)
	c.Assert(err, check.IsNil)
	c.Assert(mngr, check.Not(check.IsNil))

	type nodeState struct {
		cn           *v2.CiliumNode
		name         string
		instanceName string
	}

	state := make([]*nodeState, 100)

	for i := range state {
		s := &nodeState{name: fmt.Sprintf("node%d", i), instanceName: fmt.Sprintf("i-%d", i)}
		s.cn = newCiliumNode(s.name, s.instanceName, "m4.large", "us-west-1", "vpc-1", 1, 10, 0, 0)
		state[i] = s
		mngr.Update(s.cn)
	}

	for _, s := range state {
		c.Assert(testutils.WaitUntil(func() bool { return reachedAddressesNeeded(mngr, s.name, 0) }, 5*time.Second), check.IsNil)

		node := mngr.Get(s.name)
		c.Assert(node, check.Not(check.IsNil))
		c.Assert(node.stats.availableIPs, check.Equals, 10)
		c.Assert(node.stats.usedIPs, check.Equals, 0)
	}
}