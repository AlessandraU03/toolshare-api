package pubsub

import (
	"sync"

	"github.com/google/uuid"
)

var (
	mu        sync.RWMutex
	listeners = make(map[uuid.UUID]map[chan struct{}]struct{})
)

// Subscribe registra un canal receptor para las notificaciones de un ID de renta específico.
// Retorna el canal y una función para cancelar la suscripción.
func Subscribe(rentalID uuid.UUID) (chan struct{}, func()) {
	mu.Lock()
	defer mu.Unlock()

	ch := make(chan struct{}, 1)
	if _, ok := listeners[rentalID]; !ok {
		listeners[rentalID] = make(map[chan struct{}]struct{})
	}
	listeners[rentalID][ch] = struct{}{}

	unsubscribe := func() {
		mu.Lock()
		defer mu.Unlock()
		if m, ok := listeners[rentalID]; ok {
			delete(m, ch)
			close(ch)
			if len(m) == 0 {
				delete(listeners, rentalID)
			}
		}
	}

	return ch, unsubscribe
}

// Publish avisa a todos los canales suscritos a una renta que ha habido una actualización.
func Publish(rentalID uuid.UUID) {
	mu.RLock()
	defer mu.RUnlock()

	if m, ok := listeners[rentalID]; ok {
		for ch := range m {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
}
