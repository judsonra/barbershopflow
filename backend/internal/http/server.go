package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/example/barberflow/backend/internal/auth"
	"github.com/example/barberflow/backend/internal/domain"
	"github.com/example/barberflow/backend/internal/notify"
)

type Store interface {
	ListServices(ctx context.Context, tenantID string) ([]domain.Service, error)
	CreateService(ctx context.Context, tenantID string, item domain.Service) (domain.Service, error)
	ListProfessionals(ctx context.Context, tenantID string) ([]domain.Professional, error)
	CreateProfessional(ctx context.Context, tenantID string, item domain.Professional) (domain.Professional, error)
	ListCustomers(ctx context.Context, tenantID string) ([]domain.Customer, error)
	CreateCustomer(ctx context.Context, tenantID string, item domain.Customer) (domain.Customer, error)
	GetCustomerByID(ctx context.Context, tenantID, id string) (domain.Customer, error)
	UpdateCustomerPhone(ctx context.Context, tenantID, customerID, phone string) error
	ListAppointments(ctx context.Context, tenantID, customerID, professionalID string, from, to time.Time) ([]domain.Appointment, error)
	CreateAppointment(ctx context.Context, tenantID, status string, item domain.Appointment) (domain.Appointment, error)
	UpdateAppointmentStatus(ctx context.Context, tenantID, id, status string) (domain.Appointment, error)
	GetAppointmentProfessionalID(ctx context.Context, tenantID, id string) (string, error)
	GetUserByEmail(context.Context, string) (domain.User, error)
	GetUserByPhone(context.Context, string) (domain.User, error)
	GetUserByGoogleID(context.Context, string) (domain.User, error)
	GetUserByFacebookID(context.Context, string) (domain.User, error)
	GetUserByID(context.Context, string) (domain.User, error)
	CreateClientUser(context.Context, domain.User) (domain.User, error)
	LinkGoogleID(context.Context, string, string) error
	LinkFacebookID(context.Context, string, string) error
	SetPassword(context.Context, string, string) error
	ResetLoginAttempts(context.Context, string) error
	IncrementFailedLogin(context.Context, string) (bool, error)
	UpsertClientCredentials(ctx context.Context, tenantID, customerID, name, phone, passwordHash string) (domain.User, error)
	GetProfessionalByID(ctx context.Context, tenantID, id string) (domain.Professional, error)
	UpdateProfessionalPhone(ctx context.Context, tenantID, professionalID, phone string) error
	UpsertProfessionalCredentials(ctx context.Context, tenantID, professionalID, name, phone, passwordHash string) (domain.User, error)
	GetTenantBySlug(ctx context.Context, slug string) (domain.Tenant, error)
	TenantNameAvailable(ctx context.Context, name string) (bool, error)
	ListTenants(ctx context.Context) ([]domain.Tenant, error)
	GetFirstManagerByTenant(ctx context.Context, tenantID string) (domain.User, error)
	GetTenantByID(ctx context.Context, id string) (domain.Tenant, error)
	UpdateTenantSettings(ctx context.Context, tenantID string, selfSchedulingEnabled, autoConfirmAppointments bool) (domain.Tenant, error)
	GetProfessionalSchedule(ctx context.Context, tenantID, professionalID string) ([]domain.ScheduleEntry, error)
	SetProfessionalSchedule(ctx context.Context, tenantID, professionalID string, entries []domain.ScheduleEntry) ([]domain.ScheduleEntry, error)
	ListTimeOff(ctx context.Context, tenantID, professionalID string, from, to time.Time) ([]domain.TimeOff, error)
	CreateTimeOff(ctx context.Context, tenantID, professionalID string, item domain.TimeOff) (domain.TimeOff, error)
	DeleteTimeOff(ctx context.Context, tenantID, professionalID, id string) error
	CreateTenantWithManager(ctx context.Context, tenantName, slug, managerName, email, passwordHash string) (domain.Tenant, domain.User, error)
}

type Config struct {
	Origins     string
	Tokenizer   *auth.Tokenizer
	Google      *auth.SocialProvider
	Facebook    *auth.SocialProvider
	Notifier    notify.Notifier
	FrontendURL string
}

type server struct {
	store Store
	cfg   Config
}

type ctxKey string

const claimsContextKey ctxKey = "claims"

func New(store Store, cfg Config) http.Handler {
	s := &server{store: store, cfg: cfg}

	public := http.NewServeMux()
	public.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	public.HandleFunc("GET /api/v1/tenants/availability", s.checkTenantName)
	public.HandleFunc("POST /api/v1/tenants", s.createTenant)
	public.HandleFunc("POST /api/v1/auth/login", s.login)
	public.HandleFunc("POST /api/v1/auth/refresh", s.refresh)
	public.HandleFunc("POST /api/v1/auth/recover", s.recoverPhone)
	public.HandleFunc("GET /api/v1/auth/google/start", s.oauthStart(s.cfg.Google))
	public.HandleFunc("GET /api/v1/auth/google/callback", s.oauthCallback(s.cfg.Google, "google"))
	public.HandleFunc("GET /api/v1/auth/facebook/start", s.oauthStart(s.cfg.Facebook))
	public.HandleFunc("GET /api/v1/auth/facebook/callback", s.oauthCallback(s.cfg.Facebook, "facebook"))

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/auth/me", s.me)
	protected.HandleFunc("GET /api/v1/admin/tenants", s.requireRole(domain.RoleSuperAdmin, s.listTenantsAdmin))
	protected.HandleFunc("POST /api/v1/admin/tenants/{id}/impersonate", s.requireRole(domain.RoleSuperAdmin, s.impersonateTenant))
	protected.HandleFunc("GET /api/v1/tenant", s.getTenant)
	protected.HandleFunc("PATCH /api/v1/tenant", s.requireRole(domain.RoleManager, s.updateTenant))
	protected.HandleFunc("GET /api/v1/services", s.listServices)
	protected.HandleFunc("POST /api/v1/services", s.requireRole(domain.RoleManager, s.createService))
	protected.HandleFunc("GET /api/v1/professionals", s.listProfessionals)
	protected.HandleFunc("POST /api/v1/professionals", s.requireRole(domain.RoleManager, s.createProfessional))
	protected.HandleFunc("GET /api/v1/professionals/{id}/schedule", s.getProfessionalSchedule)
	protected.HandleFunc("PUT /api/v1/professionals/{id}/schedule", s.requireOwnerOrManager(s.setProfessionalSchedule))
	protected.HandleFunc("GET /api/v1/professionals/{id}/time-off", s.listTimeOff)
	protected.HandleFunc("POST /api/v1/professionals/{id}/time-off", s.requireOwnerOrManager(s.createTimeOff))
	protected.HandleFunc("DELETE /api/v1/professionals/{id}/time-off/{blockId}", s.requireOwnerOrManager(s.deleteTimeOff))
	protected.HandleFunc("POST /api/v1/professionals/{id}/credentials", s.requireRole(domain.RoleManager, s.grantProfessionalAccess))
	protected.HandleFunc("GET /api/v1/customers", s.requireStaff(s.listCustomers))
	protected.HandleFunc("POST /api/v1/customers", s.requireStaff(s.createCustomer))
	protected.HandleFunc("POST /api/v1/customers/{id}/credentials", s.requireStaff(s.grantCustomerAccess))
	protected.HandleFunc("GET /api/v1/appointments", s.listAppointments)
	protected.HandleFunc("POST /api/v1/appointments", s.createAppointment)
	protected.HandleFunc("PATCH /api/v1/appointments/{id}/status", s.updateStatus)

	public.Handle("/", s.requireAuth(protected))
	return recoverer(logger(cors(public, cfg.Origins)))
}

var slugPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)
var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

// checkTenantName backs the real-time availability check on the signup
// form. Public and best-effort by nature (a name-taken race is still
// possible between this check and the real POST /tenants — that path
// stays authoritative via the unique index).
func (s *server) checkTenantName(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeJSON(w, http.StatusOK, map[string]bool{"available": false})
		return
	}
	available, err := s.store.TenantNameAvailable(r.Context(), name)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"available": available})
}

// createTenant is the self-service onboarding flow: a new barbershop signs
// itself up, no invite or manual provisioning needed. Creates the tenant
// and its first manager account atomically and logs the manager in right
// away, same response shape as /auth/login.
func (s *server) createTenant(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TenantName  string `json:"tenant_name"`
		Slug        string `json:"slug"`
		ManagerName string `json:"manager_name"`
		Email       string `json:"email"`
		Password    string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	body.TenantName = strings.TrimSpace(body.TenantName)
	body.Slug = strings.ToLower(strings.TrimSpace(body.Slug))
	body.ManagerName = strings.TrimSpace(body.ManagerName)
	body.Email = strings.TrimSpace(body.Email)
	if body.TenantName == "" || body.ManagerName == "" || body.Email == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "nome da barbearia, nome do gestor e e-mail são obrigatórios")
		return
	}
	if !slugPattern.MatchString(body.Slug) {
		writeError(w, http.StatusBadRequest, "validation_error", "slug deve conter apenas letras minúsculas, números e hífen")
		return
	}
	if len(body.Password) < 8 {
		writeError(w, http.StatusBadRequest, "validation_error", "senha deve ter ao menos 8 caracteres")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		handleError(w, err)
		return
	}
	_, manager, err := s.store.CreateTenantWithManager(r.Context(), body.TenantName, body.Slug, body.ManagerName, body.Email, hash)
	if err != nil {
		handleError(w, err)
		return
	}
	s.issueTokens(w, manager)
}

func (s *server) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email    string `json:"email"`
		Phone    string `json:"phone"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Phone) != "" {
		s.loginByPhone(w, r, strings.TrimSpace(body.Phone), body.Password)
		return
	}
	user, err := s.store.GetUserByEmail(r.Context(), strings.TrimSpace(body.Email))
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "e-mail ou senha inválidos")
		return
	}
	if err != nil {
		handleError(w, err)
		return
	}
	if !user.Active || !auth.CheckPassword(user.PasswordHash, body.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "e-mail ou senha inválidos")
		return
	}
	s.issueTokens(w, user)
}

// loginByPhone is the password path for clients without e-mail. Unlike
// e-mail login it is rate limited: domain.MaxLoginAttempts wrong passwords
// lock the account until the phone recovery flow (POST /auth/recover)
// issues a new password.
func (s *server) loginByPhone(w http.ResponseWriter, r *http.Request, phone, password string) {
	user, err := s.store.GetUserByPhone(r.Context(), phone)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "celular ou senha inválidos")
		return
	}
	if err != nil {
		handleError(w, err)
		return
	}
	if !user.Active {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "celular ou senha inválidos")
		return
	}
	if user.Locked() {
		writeLockedError(w)
		return
	}
	if !auth.CheckPassword(user.PasswordHash, password) {
		locked, err := s.store.IncrementFailedLogin(r.Context(), user.ID)
		if err != nil {
			handleError(w, err)
			return
		}
		if locked {
			writeLockedError(w)
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "celular ou senha inválidos")
		return
	}
	if err := s.store.ResetLoginAttempts(r.Context(), user.ID); err != nil {
		handleError(w, err)
		return
	}
	s.issueTokens(w, user)
}

func writeLockedError(w http.ResponseWriter) {
	writeError(w, http.StatusLocked, "account_locked",
		"conta bloqueada apos muitas tentativas incorretas; use a recuperacao por SMS/WhatsApp")
}

// recoverPhone issues a brand-new temporary password to a client's or
// professional's phone (the two roles that log in by phone+password),
// clearing any lockout. The response never reveals whether the phone is
// registered, to avoid leaking which numbers have an account.
func (s *server) recoverPhone(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Phone   string `json:"phone"`
		Channel string `json:"channel"`
	}
	if !decode(w, r, &body) {
		return
	}
	phone := strings.TrimSpace(body.Phone)
	if phone == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "celular é obrigatório")
		return
	}
	channel := notify.ChannelWhatsApp
	if body.Channel == notify.ChannelSMS {
		channel = notify.ChannelSMS
	}

	user, err := s.store.GetUserByPhone(r.Context(), phone)
	if err == nil && (user.Role == domain.RoleClient || user.Role == domain.RoleProfessional) {
		if password, hashErr := auth.GenerateTempPassword(); hashErr == nil {
			if hash, hashErr := auth.HashPassword(password); hashErr == nil {
				if setErr := s.store.SetPassword(r.Context(), user.ID, hash); setErr == nil {
					if sendErr := s.cfg.Notifier.SendPassword(r.Context(), phone, password, channel); sendErr != nil {
						log.Printf("notify: failed to send recovery password: %v", sendErr)
					}
				}
			}
		}
	} else if err != nil && !errors.Is(err, domain.ErrNotFound) {
		log.Printf("recover phone lookup: %v", err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Se o número estiver cadastrado, uma nova senha foi enviada."})
}

// grantCustomerAccess is how a barber/gestor gives a customer without
// e-mail a phone login: it (re)generates a password and sends it via
// SMS/WhatsApp. Safe to call again later to reset a forgotten/locked
// password — it always issues a brand-new one.
func (s *server) grantCustomerAccess(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Phone   string `json:"phone"`
		Channel string `json:"channel"`
	}
	if !decode(w, r, &body) {
		return
	}
	claims, _ := claimsFromContext(r)
	customerID := r.PathValue("id")
	customer, err := s.store.GetCustomerByID(r.Context(), claims.TenantID, customerID)
	if err != nil {
		handleError(w, err)
		return
	}
	phone := strings.TrimSpace(body.Phone)
	if phone == "" {
		phone = customer.Phone
	}
	if phone == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "celular é obrigatório")
		return
	}
	if phone != customer.Phone {
		if err := s.store.UpdateCustomerPhone(r.Context(), claims.TenantID, customerID, phone); err != nil {
			handleError(w, err)
			return
		}
	}
	channel := notify.ChannelWhatsApp
	if body.Channel == notify.ChannelSMS {
		channel = notify.ChannelSMS
	}

	password, err := auth.GenerateTempPassword()
	if err != nil {
		handleError(w, err)
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		handleError(w, err)
		return
	}
	if _, err := s.store.UpsertClientCredentials(r.Context(), claims.TenantID, customerID, customer.Name, phone, hash); err != nil {
		handleError(w, err)
		return
	}
	if err := s.cfg.Notifier.SendPassword(r.Context(), phone, password, channel); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Senha enviada com sucesso."})
}

// grantProfessionalAccess mirrors grantCustomerAccess exactly, just for a
// professional's own login instead of a client's — same temp-password-by-
// SMS/WhatsApp mechanism, reusing UpsertProfessionalCredentials's
// ON CONFLICT (professional_id) to both create it the first time and
// regenerate a forgotten/locked password later. Manager-only: unlike
// registering a customer, handing out a coworker's login is a step up in
// sensitivity, so professionals can't grant this to themselves or others.
func (s *server) grantProfessionalAccess(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Phone   string `json:"phone"`
		Channel string `json:"channel"`
	}
	if !decode(w, r, &body) {
		return
	}
	claims, _ := claimsFromContext(r)
	professionalID := r.PathValue("id")
	professional, err := s.store.GetProfessionalByID(r.Context(), claims.TenantID, professionalID)
	if err != nil {
		handleError(w, err)
		return
	}
	phone := strings.TrimSpace(body.Phone)
	if phone == "" {
		phone = professional.Phone
	}
	if phone == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "celular é obrigatório")
		return
	}
	if phone != professional.Phone {
		if err := s.store.UpdateProfessionalPhone(r.Context(), claims.TenantID, professionalID, phone); err != nil {
			handleError(w, err)
			return
		}
	}
	channel := notify.ChannelWhatsApp
	if body.Channel == notify.ChannelSMS {
		channel = notify.ChannelSMS
	}

	password, err := auth.GenerateTempPassword()
	if err != nil {
		handleError(w, err)
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		handleError(w, err)
		return
	}
	if _, err := s.store.UpsertProfessionalCredentials(r.Context(), claims.TenantID, professionalID, professional.Name, phone, hash); err != nil {
		handleError(w, err)
		return
	}
	if err := s.cfg.Notifier.SendPassword(r.Context(), phone, password, channel); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Senha enviada com sucesso."})
}

func (s *server) refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if !decode(w, r, &body) {
		return
	}
	claims, err := s.cfg.Tokenizer.Parse(body.RefreshToken)
	if err != nil || claims.TokenType != auth.TokenTypeRefresh {
		writeError(w, http.StatusUnauthorized, "invalid_token", "refresh token inválido ou expirado")
		return
	}
	user, err := s.store.GetUserByID(r.Context(), claims.UserID)
	if err != nil || !user.Active {
		writeError(w, http.StatusUnauthorized, "invalid_token", "refresh token inválido ou expirado")
		return
	}
	s.issueTokens(w, user)
}

func (s *server) issueTokens(w http.ResponseWriter, user domain.User) {
	access, err := s.cfg.Tokenizer.GenerateAccessToken(user)
	if err != nil {
		handleError(w, err)
		return
	}
	refresh, err := s.cfg.Tokenizer.GenerateRefreshToken(user)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  access,
		"refresh_token": refresh,
		"user":          user,
	})
}

func (s *server) me(w http.ResponseWriter, r *http.Request) {
	claims, ok := claimsFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "autenticação necessária")
		return
	}
	user, err := s.store.GetUserByID(r.Context(), claims.UserID)
	respond(w, user, err)
}

func (s *server) listTenantsAdmin(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListTenants(r.Context())
	respond(w, items, err)
}

// impersonateTenant is how a superadmin "accesses" a barbershop: rather
// than bypassing tenant scoping across every endpoint, it hands back a
// normal access/refresh token pair for that tenant's own manager account,
// so everything downstream (every other handler in this file) works
// completely unchanged.
func (s *server) impersonateTenant(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFromContext(r)
	tenantID := r.PathValue("id")
	manager, err := s.store.GetFirstManagerByTenant(r.Context(), tenantID)
	if err != nil {
		handleError(w, err)
		return
	}
	log.Printf("superadmin %s impersonating tenant %s as manager %s", claims.UserID, tenantID, manager.ID)
	s.issueTokens(w, manager)
}

// getTenant is used by the client app to know whether self-scheduling is
// available before showing the "suggest a time" flow, and by staff to see
// their own settings.
func (s *server) getTenant(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFromContext(r)
	tenant, err := s.store.GetTenantByID(r.Context(), claims.TenantID)
	respond(w, tenant, err)
}

func (s *server) updateTenant(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SelfSchedulingEnabled   bool `json:"self_scheduling_enabled"`
		AutoConfirmAppointments bool `json:"auto_confirm_appointments"`
	}
	if !decode(w, r, &body) {
		return
	}
	claims, _ := claimsFromContext(r)
	tenant, err := s.store.UpdateTenantSettings(r.Context(), claims.TenantID, body.SelfSchedulingEnabled, body.AutoConfirmAppointments)
	respond(w, tenant, err)
}

// oauthStart redirects to the provider's consent screen. The CSRF state is
// a self-verifying signed token (see auth.SignState) so no server-side
// session is needed between the start and callback requests. An optional
// ?tenant=<slug> identifies which barbershop a brand-new client is signing
// up into — carried through the state, since existing accounts (staff or
// returning clients) already know their tenant from the matched user row.
func (s *server) oauthStart(provider *auth.SocialProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !provider.Configured() {
			writeError(w, http.StatusNotImplemented, "oauth_not_configured", "login social não configurado neste ambiente")
			return
		}
		tenantID := ""
		if slug := r.URL.Query().Get("tenant"); slug != "" {
			tenant, err := s.store.GetTenantBySlug(r.Context(), slug)
			if err != nil {
				handleError(w, err)
				return
			}
			tenantID = tenant.ID
		}
		state, err := auth.SignState(s.cfg.Tokenizer.Secret(), tenantID)
		if err != nil {
			handleError(w, err)
			return
		}
		http.Redirect(w, r, provider.AuthCodeURL(state), http.StatusFound)
	}
}

// oauthCallback exchanges the authorization code, finds or creates the
// matching user (see findOrCreateSocialUser), and hands the browser back to
// the frontend with fresh tokens in the URL fragment — never sent to any
// server, so it doesn't end up in access logs.
func (s *server) oauthCallback(provider *auth.SocialProvider, providerName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !provider.Configured() {
			writeError(w, http.StatusNotImplemented, "oauth_not_configured", "login social não configurado neste ambiente")
			return
		}
		tenantID, err := auth.VerifyState(s.cfg.Tokenizer.Secret(), r.URL.Query().Get("state"), 10*time.Minute)
		if err != nil {
			s.redirectWithError(w, r, "state inválido ou expirado")
			return
		}
		info, err := provider.Exchange(r.Context(), r.URL.Query().Get("code"))
		if err != nil {
			log.Printf("oauth exchange (%s): %v", providerName, err)
			s.redirectWithError(w, r, "não foi possível concluir o login")
			return
		}
		user, err := s.findOrCreateSocialUser(r.Context(), providerName, tenantID, info)
		if err != nil {
			log.Printf("oauth upsert user (%s): %v", providerName, err)
			s.redirectWithError(w, r, "não foi possível concluir o login")
			return
		}
		access, err := s.cfg.Tokenizer.GenerateAccessToken(user)
		if err != nil {
			s.redirectWithError(w, r, "não foi possível concluir o login")
			return
		}
		refresh, err := s.cfg.Tokenizer.GenerateRefreshToken(user)
		if err != nil {
			s.redirectWithError(w, r, "não foi possível concluir o login")
			return
		}
		target := fmt.Sprintf("%s/#access_token=%s&refresh_token=%s",
			strings.TrimSuffix(s.cfg.FrontendURL, "/"), url.QueryEscape(access), url.QueryEscape(refresh))
		http.Redirect(w, r, target, http.StatusFound)
	}
}

func (s *server) redirectWithError(w http.ResponseWriter, r *http.Request, message string) {
	target := fmt.Sprintf("%s/#auth_error=%s", strings.TrimSuffix(s.cfg.FrontendURL, "/"), url.QueryEscape(message))
	http.Redirect(w, r, target, http.StatusFound)
}

// findOrCreateSocialUser links a Google/Facebook account to an existing
// user matched by provider id, then by e-mail (this is how a manager or
// professional recovers access without a "forgot password" flow: sign in
// with the same e-mail via Google/Facebook). If no account matches at all,
// a brand-new client (customer + login) is self-registered into tenantID
// (resolved from ?tenant=<slug> at oauthStart) — never a manager or
// professional, those are always provisioned by staff. Self-registration
// without a tenant is rejected: there's no barbershop to enroll into.
func (s *server) findOrCreateSocialUser(ctx context.Context, providerName, tenantID string, info auth.OAuthUserInfo) (domain.User, error) {
	lookup, link := s.store.GetUserByGoogleID, s.store.LinkGoogleID
	if providerName == "facebook" {
		lookup, link = s.store.GetUserByFacebookID, s.store.LinkFacebookID
	}

	user, err := lookup(ctx, info.ProviderID)
	if err == nil {
		return user, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, err
	}

	user, err = s.store.GetUserByEmail(ctx, info.Email)
	if err == nil {
		return user, link(ctx, user.ID, info.ProviderID)
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, err
	}

	if tenantID == "" {
		return domain.User{}, fmt.Errorf("cadastro social exige a barbearia (?tenant=<slug>)")
	}
	customer, err := s.store.CreateCustomer(ctx, tenantID, domain.Customer{Name: info.Name, Email: info.Email})
	if err != nil {
		return domain.User{}, err
	}
	newUser := domain.User{TenantID: tenantID, Name: info.Name, Email: info.Email, Role: domain.RoleClient, CustomerID: customer.ID}
	if providerName == "google" {
		newUser.GoogleID = info.ProviderID
	} else {
		newUser.FacebookID = info.ProviderID
	}
	return s.store.CreateClientUser(ctx, newUser)
}

func (s *server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token, found := strings.CutPrefix(header, "Bearer ")
		if !found || token == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "autenticação necessária")
			return
		}
		claims, err := s.cfg.Tokenizer.Parse(token)
		if err != nil || claims.TokenType != auth.TokenTypeAccess {
			writeError(w, http.StatusUnauthorized, "unauthorized", "token inválido ou expirado")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), claimsContextKey, claims)))
	})
}

func (s *server) requireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r)
		if !ok || claims.Role != role {
			writeError(w, http.StatusForbidden, "forbidden", "sem permissão para esta ação")
			return
		}
		next(w, r)
	}
}

// requireStaff allows manager and professional (the "barbeiro"), but not
// client accounts, matching who is allowed to register/grant access to
// customers.
func (s *server) requireStaff(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r)
		if !ok || claims.Role == domain.RoleClient {
			writeError(w, http.StatusForbidden, "forbidden", "sem permissão para esta ação")
			return
		}
		next(w, r)
	}
}

// requireOwnerOrManager allows a manager to manage any professional's
// schedule/time-off, and a professional to manage only their own
// ({id} path value must match claims.ProfessionalID).
func (s *server) requireOwnerOrManager(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromContext(r)
		isOwner := claims.Role == domain.RoleProfessional && claims.ProfessionalID == r.PathValue("id")
		if !ok || (claims.Role != domain.RoleManager && !isOwner) {
			writeError(w, http.StatusForbidden, "forbidden", "sem permissão para esta ação")
			return
		}
		next(w, r)
	}
}

func claimsFromContext(r *http.Request) (*auth.Claims, bool) {
	claims, ok := r.Context().Value(claimsContextKey).(*auth.Claims)
	return claims, ok
}

func (s *server) listServices(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFromContext(r)
	items, err := s.store.ListServices(r.Context(), claims.TenantID)
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
	claims, _ := claimsFromContext(r)
	created, err := s.store.CreateService(r.Context(), claims.TenantID, item)
	respondCreated(w, created, err)
}
func (s *server) listProfessionals(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFromContext(r)
	items, err := s.store.ListProfessionals(r.Context(), claims.TenantID)
	respond(w, items, err)
}
func (s *server) createProfessional(w http.ResponseWriter, r *http.Request) {
	var item domain.Professional
	if !decode(w, r, &item) {
		return
	}
	item.Name = strings.TrimSpace(item.Name)
	item.Email = strings.TrimSpace(item.Email)
	item.CPF = domain.DigitsOnly(item.CPF)
	if item.Name == "" {
		writeError(w, 400, "validation_error", "nome é obrigatório")
		return
	}
	if item.Email != "" && !emailPattern.MatchString(item.Email) {
		writeError(w, 400, "validation_error", "e-mail inválido")
		return
	}
	if item.CPF != "" && !domain.ValidCPF(item.CPF) {
		writeError(w, 400, "validation_error", "CPF inválido")
		return
	}
	claims, _ := claimsFromContext(r)
	created, err := s.store.CreateProfessional(r.Context(), claims.TenantID, item)
	respondCreated(w, created, err)
}

func (s *server) getProfessionalSchedule(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFromContext(r)
	items, err := s.store.GetProfessionalSchedule(r.Context(), claims.TenantID, r.PathValue("id"))
	respond(w, items, err)
}

func (s *server) setProfessionalSchedule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Entries []domain.ScheduleEntry `json:"entries"`
	}
	if !decode(w, r, &body) {
		return
	}
	seen := map[int]bool{}
	for _, entry := range body.Entries {
		if entry.Weekday < 0 || entry.Weekday > 6 || entry.StartMinute < 0 || entry.EndMinute <= entry.StartMinute || entry.EndMinute > 1440 {
			writeError(w, http.StatusBadRequest, "validation_error", "jornada inválida: dia da semana e horários devem ser válidos")
			return
		}
		if seen[entry.Weekday] {
			writeError(w, http.StatusBadRequest, "validation_error", "dia da semana repetido")
			return
		}
		seen[entry.Weekday] = true
	}
	claims, _ := claimsFromContext(r)
	items, err := s.store.SetProfessionalSchedule(r.Context(), claims.TenantID, r.PathValue("id"), body.Entries)
	respond(w, items, err)
}

func (s *server) listTimeOff(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	from, err := parseTime(r.URL.Query().Get("from"), now.AddDate(0, 0, -1))
	if err != nil {
		writeError(w, 400, "validation_error", "from inválido")
		return
	}
	to, err := parseTime(r.URL.Query().Get("to"), from.AddDate(0, 3, 0))
	if err != nil || !to.After(from) {
		writeError(w, 400, "validation_error", "to inválido")
		return
	}
	claims, _ := claimsFromContext(r)
	items, err := s.store.ListTimeOff(r.Context(), claims.TenantID, r.PathValue("id"), from, to)
	respond(w, items, err)
}

func (s *server) createTimeOff(w http.ResponseWriter, r *http.Request) {
	var item domain.TimeOff
	if !decode(w, r, &item) {
		return
	}
	if item.StartsAt.IsZero() || item.EndsAt.IsZero() || !item.EndsAt.After(item.StartsAt) {
		writeError(w, http.StatusBadRequest, "validation_error", "início e fim válidos são obrigatórios")
		return
	}
	claims, _ := claimsFromContext(r)
	created, err := s.store.CreateTimeOff(r.Context(), claims.TenantID, r.PathValue("id"), item)
	respondCreated(w, created, err)
}

func (s *server) deleteTimeOff(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFromContext(r)
	err := s.store.DeleteTimeOff(r.Context(), claims.TenantID, r.PathValue("id"), r.PathValue("blockId"))
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Bloqueio removido."})
}

func (s *server) listCustomers(w http.ResponseWriter, r *http.Request) {
	claims, _ := claimsFromContext(r)
	items, err := s.store.ListCustomers(r.Context(), claims.TenantID)
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
	claims, _ := claimsFromContext(r)
	created, err := s.store.CreateCustomer(r.Context(), claims.TenantID, item)
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
	claims, _ := claimsFromContext(r)
	customerFilter := ""
	professionalFilter := ""
	switch claims.Role {
	case domain.RoleClient:
		customerFilter = claims.CustomerID
	case domain.RoleProfessional:
		professionalFilter = claims.ProfessionalID
	}
	items, err := s.store.ListAppointments(r.Context(), claims.TenantID, customerFilter, professionalFilter, from, to)
	respond(w, items, err)
}

// createAppointment is shared by staff (booking for any customer, always
// starts 'scheduled') and clients suggesting their own time (autoagendamento):
// gated by the tenant's self_scheduling_enabled, forced to their own
// customer_id, and starts 'confirmed' or 'scheduled' depending on
// auto_confirm_appointments — see docs/regras-de-negocio.md.
func (s *server) createAppointment(w http.ResponseWriter, r *http.Request) {
	var item domain.Appointment
	if !decode(w, r, &item) {
		return
	}
	claims, _ := claimsFromContext(r)
	status := domain.StatusScheduled
	if claims.Role == domain.RoleClient {
		tenant, err := s.store.GetTenantByID(r.Context(), claims.TenantID)
		if err != nil {
			handleError(w, err)
			return
		}
		if !tenant.SelfSchedulingEnabled {
			writeError(w, http.StatusForbidden, "forbidden", "autoagendamento não habilitado para esta barbearia")
			return
		}
		item.CustomerID = claims.CustomerID
		if tenant.AutoConfirmAppointments {
			status = domain.StatusConfirmed
		}
	}
	if item.CustomerID == "" || item.ProfessionalID == "" || item.ServiceID == "" || item.StartsAt.IsZero() {
		writeError(w, 400, "validation_error", "cliente, profissional, serviço e horário são obrigatórios")
		return
	}
	created, err := s.store.CreateAppointment(r.Context(), claims.TenantID, status, item)
	respondCreated(w, created, err)
}
func (s *server) updateStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status string `json:"status"`
	}
	if !decode(w, r, &body) {
		return
	}
	id := r.PathValue("id")
	claims, _ := claimsFromContext(r)
	if claims.Role == domain.RoleClient {
		writeError(w, http.StatusForbidden, "forbidden", "cliente não pode alterar o status do agendamento; aguarde a confirmação do profissional")
		return
	}
	if claims.Role == domain.RoleProfessional {
		professionalID, err := s.store.GetAppointmentProfessionalID(r.Context(), claims.TenantID, id)
		if err != nil {
			handleError(w, err)
			return
		}
		if professionalID != claims.ProfessionalID {
			writeError(w, http.StatusForbidden, "forbidden", "sem permissão para alterar este agendamento")
			return
		}
	}
	item, err := s.store.UpdateAppointmentStatus(r.Context(), claims.TenantID, id, body.Status)
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
	case errors.Is(err, domain.ErrForbidden):
		writeError(w, 403, "forbidden", err.Error())
	case errors.Is(err, domain.ErrConflict):
		writeError(w, 409, "conflict", "já existe um cadastro com esse identificador (e-mail, CPF, slug ou telefone)")
	case errors.Is(err, domain.ErrOutsideWorkingHours):
		writeError(w, 409, "outside_working_hours", "horário fora da jornada de trabalho do profissional")
	case errors.Is(err, domain.ErrTimeBlocked):
		writeError(w, 409, "time_blocked", "horário bloqueado (ausência, folga ou viagem)")
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
