package forjitree

import (
	"sync"
	"time"
)

type watcher struct {
	watcherId        string
	tree             *Tree
	extractTimestamp time.Time
	mu               sync.Mutex
}

func newWatcher(watcherId string) *watcher {
	w := &watcher{
		watcherId:        watcherId,
		tree:             New(),
		extractTimestamp: time.Now(),
	}
	w.tree.created = true
	return w
}

func (w *watcher) collectChanges(changes any) {
	w.mu.Lock()
	w.tree.Set(changes, true)
	w.mu.Unlock()
}

func (w *watcher) extractChanges() any {
	w.mu.Lock()
	w.extractTimestamp = time.Now()
	result := w.tree.GetValue()
	w.tree.rootNode = newNode(w.tree, nil, "")
	w.mu.Unlock()
	return result
}
