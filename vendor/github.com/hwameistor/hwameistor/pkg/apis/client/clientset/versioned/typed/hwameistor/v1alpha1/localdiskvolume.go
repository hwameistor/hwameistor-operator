// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	scheme "github.com/hwameistor/hwameistor/pkg/apis/client/clientset/versioned/scheme"
	v1alpha1 "github.com/hwameistor/hwameistor/pkg/apis/hwameistor/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// LocalDiskVolumesGetter has a method to return a LocalDiskVolumeInterface.
// A group's client should implement this interface.
type LocalDiskVolumesGetter interface {
	LocalDiskVolumes() LocalDiskVolumeInterface
}

// LocalDiskVolumeInterface has methods to work with LocalDiskVolume resources.
type LocalDiskVolumeInterface interface {
	Create(ctx context.Context, localDiskVolume *v1alpha1.LocalDiskVolume, opts v1.CreateOptions) (*v1alpha1.LocalDiskVolume, error)
	Update(ctx context.Context, localDiskVolume *v1alpha1.LocalDiskVolume, opts v1.UpdateOptions) (*v1alpha1.LocalDiskVolume, error)
	UpdateStatus(ctx context.Context, localDiskVolume *v1alpha1.LocalDiskVolume, opts v1.UpdateOptions) (*v1alpha1.LocalDiskVolume, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.LocalDiskVolume, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.LocalDiskVolumeList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.LocalDiskVolume, err error)
	LocalDiskVolumeExpansion
}

// localDiskVolumes implements LocalDiskVolumeInterface
type localDiskVolumes struct {
	client rest.Interface
}

// newLocalDiskVolumes returns a LocalDiskVolumes
func newLocalDiskVolumes(c *HwameistorV1alpha1Client) *localDiskVolumes {
	return &localDiskVolumes{
		client: c.RESTClient(),
	}
}

// Get takes name of the localDiskVolume, and returns the corresponding localDiskVolume object, and an error if there is any.
func (c *localDiskVolumes) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.LocalDiskVolume, err error) {
	result = &v1alpha1.LocalDiskVolume{}
	err = c.client.Get().
		Resource("localdiskvolumes").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of LocalDiskVolumes that match those selectors.
func (c *localDiskVolumes) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.LocalDiskVolumeList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.LocalDiskVolumeList{}
	err = c.client.Get().
		Resource("localdiskvolumes").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested localDiskVolumes.
func (c *localDiskVolumes) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Resource("localdiskvolumes").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a localDiskVolume and creates it.  Returns the server's representation of the localDiskVolume, and an error, if there is any.
func (c *localDiskVolumes) Create(ctx context.Context, localDiskVolume *v1alpha1.LocalDiskVolume, opts v1.CreateOptions) (result *v1alpha1.LocalDiskVolume, err error) {
	result = &v1alpha1.LocalDiskVolume{}
	err = c.client.Post().
		Resource("localdiskvolumes").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(localDiskVolume).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a localDiskVolume and updates it. Returns the server's representation of the localDiskVolume, and an error, if there is any.
func (c *localDiskVolumes) Update(ctx context.Context, localDiskVolume *v1alpha1.LocalDiskVolume, opts v1.UpdateOptions) (result *v1alpha1.LocalDiskVolume, err error) {
	result = &v1alpha1.LocalDiskVolume{}
	err = c.client.Put().
		Resource("localdiskvolumes").
		Name(localDiskVolume.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(localDiskVolume).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *localDiskVolumes) UpdateStatus(ctx context.Context, localDiskVolume *v1alpha1.LocalDiskVolume, opts v1.UpdateOptions) (result *v1alpha1.LocalDiskVolume, err error) {
	result = &v1alpha1.LocalDiskVolume{}
	err = c.client.Put().
		Resource("localdiskvolumes").
		Name(localDiskVolume.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(localDiskVolume).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the localDiskVolume and deletes it. Returns an error if one occurs.
func (c *localDiskVolumes) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Resource("localdiskvolumes").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *localDiskVolumes) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Resource("localdiskvolumes").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched localDiskVolume.
func (c *localDiskVolumes) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.LocalDiskVolume, err error) {
	result = &v1alpha1.LocalDiskVolume{}
	err = c.client.Patch(pt).
		Resource("localdiskvolumes").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
