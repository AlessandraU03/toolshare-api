package toolservice

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	toolports "github.com/yourusername/tool-inventory-api/internal/tool/ports"
)

// ticketJobStore resuelve el timeout de proxy en extract-ticket-price: el
// OCR de un ticket puede tardar decenas de segundos (arranque en frio del
// worker de PaddleOCR), y una sola peticion HTTP que dura tanto corre el
// riesgo de que algun proxy intermedio (Railway) la corte a medias aunque
// el servidor SI haya terminado bien su parte. En vez de una peticion larga,
// el cliente manda la foto, recibe un job_id de inmediato, y pregunta el
// estatus cada pocos segundos -- ninguna peticion individual dura mas de
// 1-2 segundos.
type ticketJobStatus string

const (
	ticketJobProcessing ticketJobStatus = "processing"
	ticketJobDone       ticketJobStatus = "done"
	ticketJobFailed     ticketJobStatus = "failed"
)

type ticketJob struct {
	Status    ticketJobStatus
	Result    *toolports.ExtractTicketPriceOutput
	Error     string
	CreatedAt time.Time
}

type ticketJobStore struct {
	mu   sync.Mutex
	jobs map[string]*ticketJob
}

func newTicketJobStore() *ticketJobStore {
	store := &ticketJobStore{jobs: make(map[string]*ticketJob)}
	go store.limpiarPeriodicamente()
	return store
}

// start guarda los bytes ya leidos (no un io.Reader del multipart original,
// que deja de ser valido en cuanto el handler que lo recibio termina) y
// lanza el OCR real en un goroutine aparte.
func (s *ticketJobStore) start(svc *toolService, filename string, content []byte, contentType string) string {
	jobID := uuid.NewString()
	job := &ticketJob{Status: ticketJobProcessing, CreatedAt: time.Now()}

	s.mu.Lock()
	s.jobs[jobID] = job
	s.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		out, err := svc.ExtractTicketPrice(ctx, filename, bytes.NewReader(content), contentType)

		s.mu.Lock()
		defer s.mu.Unlock()
		if err != nil {
			job.Status = ticketJobFailed
			job.Error = err.Error()
			return
		}
		job.Status = ticketJobDone
		job.Result = out
	}()

	return jobID
}

func (s *ticketJobStore) get(jobID string) (*ticketJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	return job, ok
}

// limpiarPeriodicamente evita que jobs abandonados (el usuario cerro la app
// a medio proceso y nunca volvio a preguntar por el resultado) se queden en
// memoria para siempre.
func (s *ticketJobStore) limpiarPeriodicamente() {
	for {
		time.Sleep(5 * time.Minute)
		limite := time.Now().Add(-15 * time.Minute)
		s.mu.Lock()
		for id, job := range s.jobs {
			if job.CreatedAt.Before(limite) {
				delete(s.jobs, id)
			}
		}
		s.mu.Unlock()
	}
}
