/*
Copyright 2018 Rancher Labs, Inc.

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
package fake

import (
	v1alpha1 "github.com/rancher/vm/pkg/apis/ranchervm/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeARPTables implements ARPTableInterface
type FakeARPTables struct {
	Fake *FakeVirtualmachineV1alpha1
}

var arptablesResource = schema.GroupVersionResource{Group: "virtualmachine.rancher.com", Version: "v1alpha1", Resource: "arptables"}

var arptablesKind = schema.GroupVersionKind{Group: "virtualmachine.rancher.com", Version: "v1alpha1", Kind: "ARPTable"}

// Get takes name of the aRPTable, and returns the corresponding aRPTable object, and an error if there is any.
func (c *FakeARPTables) Get(name string, options v1.GetOptions) (result *v1alpha1.ARPTable, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(arptablesResource, name), &v1alpha1.ARPTable{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ARPTable), err
}

// List takes label and field selectors, and returns the list of ARPTables that match those selectors.
func (c *FakeARPTables) List(opts v1.ListOptions) (result *v1alpha1.ARPTableList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(arptablesResource, arptablesKind, opts), &v1alpha1.ARPTableList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.ARPTableList{}
	for _, item := range obj.(*v1alpha1.ARPTableList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested aRPTables.
func (c *FakeARPTables) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(arptablesResource, opts))
}

// Create takes the representation of a aRPTable and creates it.  Returns the server's representation of the aRPTable, and an error, if there is any.
func (c *FakeARPTables) Create(aRPTable *v1alpha1.ARPTable) (result *v1alpha1.ARPTable, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(arptablesResource, aRPTable), &v1alpha1.ARPTable{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ARPTable), err
}

// Update takes the representation of a aRPTable and updates it. Returns the server's representation of the aRPTable, and an error, if there is any.
func (c *FakeARPTables) Update(aRPTable *v1alpha1.ARPTable) (result *v1alpha1.ARPTable, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(arptablesResource, aRPTable), &v1alpha1.ARPTable{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ARPTable), err
}

// Delete takes name of the aRPTable and deletes it. Returns an error if one occurs.
func (c *FakeARPTables) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteAction(arptablesResource, name), &v1alpha1.ARPTable{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeARPTables) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(arptablesResource, listOptions)

	_, err := c.Fake.Invokes(action, &v1alpha1.ARPTableList{})
	return err
}

// Patch applies the patch and returns the patched aRPTable.
func (c *FakeARPTables) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.ARPTable, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(arptablesResource, name, data, subresources...), &v1alpha1.ARPTable{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.ARPTable), err
}
