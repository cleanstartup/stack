package web

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
)

type DevState struct {
	revision atomic.Int64
	mu       sync.Mutex
	listener map[chan int64]struct{}
}

func NewDevState() *DevState {
	return &DevState{listener: map[chan int64]struct{}{}}
}

func (s *DevState) Revision() int64 {
	if s == nil {
		return 0
	}
	return s.revision.Load()
}

func (s *DevState) MarkBuilt() int64 {
	if s == nil {
		return 0
	}
	return s.revision.Add(1)
}

func (s *DevState) Broadcast() int64 {
	if s == nil {
		return 0
	}
	rev := s.MarkBuilt()
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.listener {
		select {
		case ch <- rev:
		default:
		}
	}
	return rev
}

func (s *DevState) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s == nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := make(chan int64, 1)
	s.mu.Lock()
	s.listener[ch] = struct{}{}
	current := s.Revision()
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.listener, ch)
		s.mu.Unlock()
	}()

	_, _ = fmt.Fprintf(w, ": connected %d\n\n", current)
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case rev := <-ch:
			_, _ = fmt.Fprintf(w, "data: %d\n\n", rev)
			flusher.Flush()
		}
	}
}

func (s *DevState) Handler() http.Handler {
	return http.HandlerFunc(s.ServeHTTP)
}

func withDevState(req *http.Request, state *DevState) *http.Request {
	if req == nil || state == nil {
		return req
	}
	return req.WithContext(context.WithValue(req.Context(), devStateContextKey{}, state))
}

func withAssetManifest(req *http.Request, manifest AssetManifest) *http.Request {
	if req == nil {
		return req
	}
	return req.WithContext(context.WithValue(req.Context(), assetManifestContextKey{}, manifest))
}

func devStateFromContext(ctx context.Context) *DevState {
	if ctx == nil {
		return nil
	}
	state, _ := ctx.Value(devStateContextKey{}).(*DevState)
	return state
}

func assetManifestFromContext(ctx context.Context) AssetManifest {
	if ctx == nil {
		return AssetManifest{}
	}
	manifest, _ := ctx.Value(assetManifestContextKey{}).(AssetManifest)
	return manifest
}

func devLiveReloadScriptURL() string {
	return "/__stack/dev/events"
}
