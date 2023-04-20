package storageclass

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LocalStorageNodeWatcher struct {
	GoroutineStarted bool
}

var WatcherMutex sync.Mutex

var lsnwatcher LocalStorageNodeWatcher

var stopCh chan struct{}

func EnsureWatcherStarted(cli client.Client, clusterKey types.NamespacedName)  {
	WatcherMutex.Lock()
	if !lsnwatcher.GoroutineStarted {
		go WatchLocalStorageNodes(cli, clusterKey, stopCh)
		log.Infof("Watch goroutine started")
		lsnwatcher.GoroutineStarted = true
	}
	WatcherMutex.Unlock()
}