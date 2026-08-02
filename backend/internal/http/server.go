package http

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/example/barberflow/backend/internal/domain"
)

type Store interface {
	ListServices(context.Context) ([]domain.Service, error)
	CreateService(context.Context, domain.Service) (domain.Service, error)
	ListProfessionals(context.Context) ([]domain.Professional, error)
	CreateProfessional(context.Context, domain.Professional) (domain.Professional, error)
	ListCustomers(context.Context) ([]domain.Customer, error)
	CreateCustomer(context.Context, domain.Customer) (domain.Customer, error)
	ListAppointments(context.Context, time.Time, time.Time) ([]domain.Appointment, error)
	CreateAppointment(context.Context, domain.Appointment) (domain.Appointment, error)
	UpdateAppointmentStatus(context.Context, string, string) (domain.Appointment, error)
}

type server struct{ store Store }

func New(store Store, origins string) http.Handler {
	s := &server{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/v1/services", s.listServices)
	mux.HandleFunc("POST /api/v1/services", s.createService)
	mux.HandleFunc("GET /api/v1/professionals", s.listProfessionals)
	mux.HandleFunc("POST /api/v1/professionals", s.createProfessional)
	mux.HandleFunc("GET /api/v1/customers", s.listCustomers)
	mux.HandleFunc("POST /api/v1/customers", s.createCustomer)
	mux.HandleFunc("GET /api/v1/appointments", s.listAppointments)
	mux.HandleFunc("POST /api/v1/appointments", s.createAppointment)
	mux.HandleFunc("PATCH /api/v1/appointments/{id}/status", s.updateStatus)
	return recoverer(logger(cors(mux, origins)))
}

func (s *server) listServices(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListServices(r.Context())
	respond(w, items, err)
}
func (s *server) createService(w http.ResponseWriter, r *http.Request) {
	var item domain.Service
	if !decode(w, r, &item) {
		return
	}
	item.Name = strings.TrimSpace(item.Name)
	if item.Name == "" || item.DurationMinutes < 5 || item.DurationMinutes > 480 || item.PriceCents < 0 {
		writeError(w, http.StatusBadRequest, "validation_error", "nome, duração de 5 a 480 minutos e preço não negativo são obrigatórios")
		return
	}
	created, err := s.store.CreateService(r.Context(), item)
	respondCreated(w, created, err)
}
func (s *server) listProfessionals(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListProfessionals(r.Context())
	respond(w, items, err)
}
func (s *server) createProfessional(w http.ResponseWriter, r *http.Request) {
	var item domain.Professional
	if !decode(w, r, &item) {
		return
	}
	item.Name = strings.TrimSpace(item.Name)
	if item.Name == "" {
		writeError(w, 400, "validation_error", "nome é obrigatório")
		return
	}
	created, err := s.store.CreateProfessional(r.Context(), item)
	respondCreated(w, created, err)
}
func (s *server) listCustomers(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListCustomers(r.Context())
	respond(w, items, err)
}
func (s *server) createCustomer(w http.ResponseWriter, r *http.Request) {
	var item domain.Customer
	if !decode(w, r, &item) {
		return
	}
	item.Name = strings.TrimSpace(item.Name)
	if item.Name == "" {
		writeError(w, 400, "validation_error", "nome é obrigatório")
		return
	}
	created, err := s.store.CreateCustomer(r.Context(), item)
	respondCreated(w, created, err)
}
func (s *server) listAppointments(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	from, err := parseTime(r.URL.Query().Get("from"), time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()))
	if err != nil {
		writeError(w, 400, "validation_error", "from inválido")
		return
	}
	to, err := parseTime(r.URL.Query().Get("to"), from.AddDate(0, 0, 7))
	if err != nil || !to.After(from) {
		writeError(w, 400, "validation_error", "to inválido")
		return
	}
	items, err := s.store.ListAppointments(r.Context(), from, to)
	respond(w, items, err)
}
func (s *server) createAppointment(w http.ResponseWriter, r *http.Request) {
	var item domain.Appointment
	if !decode(w, r, &item) {
		return
	}
	if item.CustomerID == "" || item.ProfessionalID == "" || item.ServiceID == "" || item.StartsAt.IsZero() {
		writeError(w, 400, "validation_error", "cliente, profissional, serviço e horário são obrigatórios")
		return
	}
	created, err := s.store.CreateAppointment(r.Context(), item)
	respondCreated(w, created, err)
}
func (s *server) updateStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &body) {
		return
	}
	item, err := s.store.UpdateAppointmentStatus(r.Context(), r.PathValue("id"), body.Status)
	respond(w, item, err)
}

func parseTime(value string, fallback time.Time) (time.Time, error) {
	if value == "" {
		return fallback, nil
	}
	return time.Parse(time.RFC3339, value)
}
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, 400, "invalid_json", "JSON inválido: "+err.Error())
		return false
	}
	return true
}
func respondCreated(w http.ResponseWriter, value any, err error) {
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, value)
}
func respond(w http.ResponseWriter, value any, err error) {
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, 404, "not_found", err.Error())
	case errors.Is(err, domain.ErrScheduleConflict):
		writeError(w, 409, "schedule_conflict", err.Error())
	case errors.Is(err, domain.ErrInvalidTransition):
		writeError(w, 422, "invalid_transition", err.Error())
	default:
		log.Printf("request error: %v", err)
		writeError(w, 500, "internal_error", "erro interno")
	}
}
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func cors(next http.Handler, raw string) http.Handler {
	allowed := map[string]bool{}
	for _, origin := range strings.Split(raw, ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			allowed[origin] = true
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if allowed[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}
func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if value := recover(); value != nil {
				log.Printf("panic: %v", value)
				writeError(w, 500, "internal_error", "erro interno")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
