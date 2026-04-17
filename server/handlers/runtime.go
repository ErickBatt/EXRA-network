package handlers

import (
	"exra/hub"
)

var runtimeHub *hub.Hub
func SetHub(h *hub.Hub) {
	runtimeHub = h
}
