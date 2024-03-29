// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	time "time"

	versioned "github.com/hwameistor/hwameistor/pkg/apis/client/clientset/versioned"
	internalinterfaces "github.com/hwameistor/hwameistor/pkg/apis/client/informers/externalversions/internalinterfaces"
	v1alpha1 "github.com/hwameistor/hwameistor/pkg/apis/client/listers/hwameistor/v1alpha1"
	hwameistorv1alpha1 "github.com/hwameistor/hwameistor/pkg/apis/hwameistor/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// LocalVolumeConvertInformer provides access to a shared informer and lister for
// LocalVolumeConverts.
type LocalVolumeConvertInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1alpha1.LocalVolumeConvertLister
}

type localVolumeConvertInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
}

// NewLocalVolumeConvertInformer constructs a new informer for LocalVolumeConvert type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewLocalVolumeConvertInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredLocalVolumeConvertInformer(client, resyncPeriod, indexers, nil)
}

// NewFilteredLocalVolumeConvertInformer constructs a new informer for LocalVolumeConvert type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredLocalVolumeConvertInformer(client versioned.Interface, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.HwameistorV1alpha1().LocalVolumeConverts().List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.HwameistorV1alpha1().LocalVolumeConverts().Watch(context.TODO(), options)
			},
		},
		&hwameistorv1alpha1.LocalVolumeConvert{},
		resyncPeriod,
		indexers,
	)
}

func (f *localVolumeConvertInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredLocalVolumeConvertInformer(client, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *localVolumeConvertInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&hwameistorv1alpha1.LocalVolumeConvert{}, f.defaultInformer)
}

func (f *localVolumeConvertInformer) Lister() v1alpha1.LocalVolumeConvertLister {
	return v1alpha1.NewLocalVolumeConvertLister(f.Informer().GetIndexer())
}
