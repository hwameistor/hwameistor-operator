package storageclass

import (
	"sync"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LocalDiskNodeWatcher struct {
	GoroutineStarted bool
}

var ldnWatcherMutex sync.Mutex

var ldnWatcher LocalDiskNodeWatcher

var ldnWatcherStopCh chan struct{}

func EnsureLDNWatcherStarted(cli client.Client, clusterKey types.NamespacedName)  {
	ldnWatcherMutex.Lock()
	if !ldnWatcher.GoroutineStarted {
		go WatchLocalDiskNodes(cli, clusterKey, ldnWatcherStopCh)
		log.Infof("localdisknode watcher goroutine started")
		ldnWatcher.GoroutineStarted = true
	}
	ldnWatcherMutex.Unlock()
}