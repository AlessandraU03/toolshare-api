package userservice

import (
	"bytes"
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// kycJobStore resuelve el mismo problema de proxy que ya se arreglo en
// extract-ticket-price (ver ticket_job_store.go en el paquete toolservice):
// VerifyKyc puede tardar 15-20+ segundos (Haar Cascade + arranque en frio
// del worker de PaddleOCR + carga/inferencia de ArcFace), y una peticion
// HTTP tan larga corre el riesgo de que algun proxy intermedio (Railway)
// la corte a medias -- confirmado en produccion: el log de Go mostraba
// 200 OK mientras la app ya habia mostrado "validacion rechazada".
type kycJobStatus string

const (
	kycJobProcessing kycJobStatus = "processing"
	kycJobDone       kycJobStatus = "done"
	kycJobFailed     kycJobStatus = "failed"
)

type kycJob struct {
	Status    kycJobStatus
	Result    interface{}
	Error     string
	CreatedAt time.Time
}

type kycJobStore struct {
	mu   sync.Mutex
	jobs map[string]*kycJob
}

func newKycJobStore() *kycJobStore {
	store := &kycJobStore{jobs: make(map[string]*kycJob)}
	go store.limpiarPeriodicamente()
	return store
}

// start guarda los bytes de INE y selfie ya leidos (no io.Reader del
// multipart original, que deja de ser valido en cuanto el handler termina)
// y lanza la verificacion real en un goroutine aparte.
func (s *kycJobStore) start(svc *authService, ineFilename string, ineContent []byte, selfieFilename string, selfieContent []byte, curp string) string {
	jobID := uuid.NewString()
	job := &kycJob{Status: kycJobProcessing, CreatedAt: time.Now()}

	s.mu.Lock()
	s.jobs[jobID] = job
	s.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		res, err := svc.VerifyKyc(ctx, ineFilename, bytes.NewReader(ineContent), selfieFilename, bytes.NewReader(selfieContent), curp)

		s.mu.Lock()
		defer s.mu.Unlock()
		if err != nil {
			job.Status = kycJobFailed
			job.Error = err.Error()
			return
		}
		job.Status = kycJobDone
		job.Result = res
	}()

	return jobID
}

func (s *kycJobStore) get(jobID string) (*kycJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[jobID]
	return job, ok
}

// limpiarPeriodicamente evita que jobs abandonados (el usuario cerro la app
// a medio proceso) se queden en memoria para siempre.
func (s *kycJobStore) limpiarPeriodicamente() {
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
