// Copyright 2016-2019 Authors of Cilium
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

package endpoint

import "github.com/cilium/cilium/pkg/logging/logfields"

// GetContainerName returns the name of the container for the endpoint.
func (e *Endpoint) GetContainerName() string {
	e.unconditionalRLock()
	defer e.runlock()
	return e.containerName
}

// SetContainerName modifies the endpoint's container name
func (e *Endpoint) SetContainerName(name string) {
	e.unconditionalLock()
	e.containerName = name
	e.unlock()
}

// GetK8sNamespace returns the name of the pod if the endpoint represents a
// Kubernetes pod
func (e *Endpoint) GetK8sNamespace() string {
	e.unconditionalRLock()
	ns := e.K8sNamespace
	e.runlock()
	return ns
}

// SetK8sNamespace modifies the endpoint's pod name
func (e *Endpoint) SetK8sNamespace(name string) {
	e.unconditionalLock()
	e.K8sNamespace = name
	e.UpdateLogger(map[string]interface{}{
		logfields.K8sPodName: e.getK8sNamespaceAndPodName(),
	})
	e.unlock()
}

// K8sNamespaceAndPodNameIsSet returns true if the pod name is set
func (e *Endpoint) K8sNamespaceAndPodNameIsSet() bool {
	e.unconditionalLock()
	podName := e.getK8sNamespaceAndPodName()
	e.unlock()
	return podName != "" && podName != "/"
}

// GetK8sPodName returns the name of the pod if the endpoint represents a
// Kubernetes pod
func (e *Endpoint) GetK8sPodName() string {
	e.unconditionalRLock()
	k8sPodName := e.K8sPodName
	e.runlock()

	return k8sPodName
}

// HumanStringLocked returns the endpoint's most human readable identifier as string
func (e *Endpoint) HumanStringLocked() string {
	if pod := e.getK8sNamespaceAndPodName(); pod != "" {
		return pod
	}

	return e.StringID()
}

// GetK8sNamespaceAndPodName returns the corresponding namespace and pod
// name for this endpoint.
func (e *Endpoint) GetK8sNamespaceAndPodName() string {
	e.unconditionalRLock()
	defer e.runlock()

	return e.getK8sNamespaceAndPodName()
}

func (e *Endpoint) getK8sNamespaceAndPodName() string {
	return e.K8sNamespace + "/" + e.K8sPodName
}

// SetK8sPodName modifies the endpoint's pod name
func (e *Endpoint) SetK8sPodName(name string) {
	e.unconditionalLock()
	e.K8sPodName = name
	e.UpdateLogger(map[string]interface{}{
		logfields.K8sPodName: e.getK8sNamespaceAndPodName(),
	})
	e.unlock()
}

// SetContainerID modifies the endpoint's container ID
func (e *Endpoint) SetContainerID(id string) {
	e.unconditionalLock()
	e.containerID = id
	e.UpdateLogger(map[string]interface{}{
		logfields.ContainerID: e.getShortContainerID(),
	})
	e.unlock()
}

// GetContainerID returns the endpoint's container ID
func (e *Endpoint) GetContainerID() string {
	e.unconditionalRLock()
	cID := e.containerID
	e.runlock()
	return cID
}

// GetShortContainerID returns the endpoint's shortened container ID
func (e *Endpoint) GetShortContainerID() string {
	e.unconditionalRLock()
	defer e.runlock()

	return e.getShortContainerID()
}

func (e *Endpoint) getShortContainerID() string {
	if e == nil {
		return ""
	}

	caplen := 10
	if len(e.containerID) <= caplen {
		return e.containerID
	}

	return e.containerID[:caplen]

}

// SetDockerEndpointID modifies the endpoint's Docker Endpoint ID
func (e *Endpoint) SetDockerEndpointID(id string) {
	e.unconditionalLock()
	e.dockerEndpointID = id
	e.unlock()
}

func (e *Endpoint) GetDockerEndpointID() string {
	e.unconditionalRLock()
	defer e.runlock()
	return e.dockerEndpointID
}

// SetDockerNetworkID modifies the endpoint's Docker Endpoint ID
func (e *Endpoint) SetDockerNetworkID(id string) {
	e.unconditionalLock()
	e.dockerNetworkID = id
	e.unlock()
}

// GetDockerNetworkID returns the endpoint's Docker Endpoint ID
func (e *Endpoint) GetDockerNetworkID() string {
	e.unconditionalRLock()
	defer e.runlock()

	return e.dockerNetworkID
}