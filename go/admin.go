package main

import (
	"net/http"
)

type cacheReloader interface {
	Reload()
}

type adminHandler struct {
	reloader cacheReloader
}

func (a *adminHandler) Stats(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

func (a *adminHandler) ReloadCache(w http.ResponseWriter, r *http.Request) {
	a.reloader.Reload()
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("cache reload message sent"))
}
