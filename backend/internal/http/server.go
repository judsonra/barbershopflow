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
	UpdateService(ctx context.Context, tenantID, id string, item domain.Service) (domain.Service, error)
	ListProfessionals(ctx context.Context, tenantID string) ([]domain.Professional, error)
	CreateProfessional(ctx context.Context, tenantID string, item domain.Professional) (domain.Professional, error)
	UpdateProfessional(ctx context.Context, tenantID, id string, item domain.Professional) (domain.Professional, error)
	ListCustomers(ctx context.Context, tenantID string) ([]domain.Customer, error)
	SearchCustomersGlobal(ctx context.Context, query string) ([]domain.AdminCustomerMatch, error)
	CreateCustomer(ctx context.Context, tenantID string, item domain.Customer) (domain.Customer, error)
	UpdateCustomer(ctx context.Context, tenantID, id string, item domain.Customer) (domain.Customer, error)
	GetCustomerByID(ctx context.Context, tenantID, id string) (domain.Customer, error)
	UpdateCustomerPhone(ctx context.Context, tenantID, customerID, phone string) error
	ListAppointments(ctx context.Context, tenantID, customerID, professionalID string, from, to time.Time) ([]domain.Appointment, error)
	GetReport(ctx context.Context, tenantID string, from, to time.Time) (domain.Report, error)
	CreateAppointment(ctx context.Context, tenantID, status string, item domain.Appointment) (domain.Appointment, error)
	UpdateAppointmentStatus(ctx context.Context, tenantID, id, status string) (domain.Appointment, error)
	GetAppointmentProfessionalID(ctx context.Context, tenantID, id string) (string, error)
	FindIdentityByEmail(context.Context, string) (domain.Identity, error)
	FindIdentityByPhone(context.Context, string) (domain.Identity, error)
	FindIdentityByGoogleID(context.Context, string) (domain.Identity, error)
	FindIdentityByFacebookID(context.Context, string) (domain.Identity, error)
	LinkGoogleID(ctx context.Context, identityID, googleID string) error
	LinkFacebookID(ctx context.Context, identityID, facebookID string) error
	SetPassword(ctx context.Context, identityID, passwordHash string) error
	ResetLoginAttempts(ctx context.Context, identityID string) error
	IncrementFailedLogin(ctx context.Context, identityID string) (bool, error)
	GetMembershipByID(ctx context.Context, membershipID string) (domain.User, error)
	GetMembershipForIdentityAndTenant(ctx context.Context, identityID, tenantID string) (domain.User, error)
	GetMembershipForIdentity(ctx context.Context, identityID, membershipID string) (domain.User, error)
	FindMembershipByCustomerID(ctx context.Context, customerID string) (domain.User, error)
	FindMembershipByProfessionalID(ctx context.Context, professionalID string) (domain.User, error)
	ListMembershipsByIdentity(ctx context.Context, identityID string) ([]domain.MembershipOption, error)
	AttachMembership(ctx context.Context, identityID, tenantID, role, professionalID, customerID string) (domain.User, error)
	CreateIdentityWithMembership(ctx context.Context, name, email, phone, passwordHash, googleID, facebookID, tenantID, role, professionalID, customerID string) (domain.User, error)
	GetProfessionalByID(ctx context.Context, tenantID, id string) (domain.Professional, error)
	UpdateProfessionalPhone(ctx context.Context, tenantID, professionalID, phone string) error
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
	public.HandleFunc("POST /api/v1/auth/select-membership", s.selectMembership)
	public.HandleFunc("GET /api/v1/auth/google/start", s.oauthStart(s.cfg.Google))
	public.HandleFunc("GET /api/v1/auth/google/callback", s.oauthCallback(s.cfg.Google, "google"))
	public.HandleFunc("GET /api/v1/auth/facebook/start", s.oauthStart(s.cfg.Facebook))
	public.HandleFunc("GET /api/v1/auth/facebook/callback", s.oauthCallback(s.cfg.Facebook, "facebook"))

	protected := http.NewServeMux()
	protected.HandleFunc("GET /api/v1/auth/me", s.me)
	protected.HandleFunc("GET /api/v1/admin/tenants", s.requireRole(domain.RoleSuperAdmin, s.listTenantsAdmin))
	protected.HandleFunc("POST /api/v1/admin/tenants/{id}/impersonate", s.requireRole(domain.RoleSuperAdmin, s.impersonateTenant))
	protected.HandleFunc("GET /api/v1/admin/customers", s.requireRole(domain.RoleSuperAdmin, s.adminSearchCustomers))
	protected.HandleFunc("GET /api/v1/tenant", s.getTenant)
	protected.HandleFunc("PATCH /api/v1/tenant", s.requireRole(domain.RoleManager, s.updateTenant))
	protected.HandleFunc("GET /api/v1/services", s.listServices)
	protected.HandleFunc("POST /api/v1/services", s.requireRole(domain.RoleManager, s.createService))
	protected.HandleFunc("PATCH /api/v1/services/{id}", s.requireRole(domain.RoleManager, s.updateService))
	protected.HandleFunc("GET /api/v1/professionals", s.listProfessionals)
	protected.HandleFunc("POST /api/v1/professionals", s.requireRole(domain.RoleManager, s.createProfessional))
	protected.HandleFunc("PATCH /api/v1/professionals/{id}", s.requireRole(domain.RoleManager, s.updateProfessional))
	protected.HandleFunc("GET /api/v1/professionals/{id}/schedule", s.getProfessionalSchedule)
	protected.HandleFunc("PUT /api/v1/professionals/{id}/schedule", s.requireOwnerOrManager(s.setProfessionalSchedule))
	protected.HandleFunc("GET /api/v1/professionals/{id}/time-off", s.listTimeOff)
	protected.HandleFunc("POST /api/v1/professionals/{id}/time-off", s.requireOwnerOrManager(s.createTimeOff))
	protected.HandleFunc("DELETE /api/v1/professionals/{id}/time-off/{blockId}", s.requireOwnerOrManager(s.deleteTimeOff))
	protected.HandleFunc("POST /api/v1/professionals/{id}/credentials", s.requireRole(domain.RoleManager, s.grantProfessionalAccess))
	protected.HandleFunc("GET /api/v1/customers", s.requireStaff(s.listCustomers))
	protected.HandleFunc("POST /api/v1/customers", s.requireStaff(s.createCustomer))
	protected.HandleFunc("PATCH /api/v1/customers/{id}", s.requireStaff(s.updateCustomer))
	protected.HandleFunc("POST /api/v1/customers/{id}/credentials", s.requireStaff(s.grantCustomerAccess))
	protected.HandleFunc("GET /api/v1/appointments", s.listAppointments)
	protected.HandleFunc("GET /api/v1/reports", s.requireRole(domain.RoleManager, s.getReport))
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
	identity, err := s.store.FindIdentityByEmail(r.Context(), strings.TrimSpace(body.Email))
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "e-mail ou senha inválidos")
		return
	}
	if err != nil {
		handleError(w, err)
		return
	}
	if !auth.CheckPassword(identity.PasswordHash, body.Password) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "e-mail ou senha inválidos")
		return
	}
	s.resolveLogin(w, r, identity)
}

// loginByPhone is the password path for clients without e-mail. Unlike
// e-mail login it is rate limited: domain.MaxLoginAttempts wrong passwords
// lock the account until the phone recovery flow (POST /auth/recover)
// issues a new password. Rate limiting is identity-scoped, not
// membership-scoped — trying a different barbershop isn't a fresh set of
// attempts.
func (s *server) loginByPhone(w http.ResponseWriter, r *http.Request, phone, password string) {
	identity, err := s.store.FindIdentityByPhone(r.Context(), phone)
	if errors.Is(err, domain.ErrNotFound) {
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "celular ou senha inválidos")
		return
	}
	if err != nil {
		handleError(w, err)
		return
	}
	if identity.Locked() {
		writeLockedError(w)
		return
	}
	if !auth.CheckPassword(identity.PasswordHash, password) {
		locked, err := s.store.IncrementFailedLogin(r.Context(), identity.ID)
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
	if err := s.store.ResetLoginAttempts(r.Context(), identity.ID); err != nil {
		handleError(w, err)
		return
	}
	s.resolveLogin(w, r, identity)
}

// resolveLogin picks which barbershop to log an already-authenticated
// identity into: straight to a token when there's exactly one active
// membership (the common case, unchanged from before identity/membership
// were split into separate tables), invalid_credentials when there are
// none, or a short-lived pre-auth token plus the list to choose from when
// the same person has more than one — see selectMembership.
func (s *server) resolveLogin(w http.ResponseWriter, r *http.Request, identity domain.Identity) {
	memberships, err := s.store.ListMembershipsByIdentity(r.Context(), identity.ID)
	if err != nil {
		handleError(w, err)
		return
	}
	switch len(memberships) {
	case 0:
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "conta sem acesso ativo")
	case 1:
		user, err := s.store.GetMembershipByID(r.Context(), memberships[0].MembershipID)
		if err != nil {
			handleError(w, err)
			return
		}
		s.issueTokens(w, user)
	default:
		s.respondMembershipChoice(w, identity.ID, memberships)
	}
}

func (s *server) respondMembershipChoice(w http.ResponseWriter, identityID string, memberships []domain.MembershipOption) {
	preauth, err := auth.SignState(s.cfg.Tokenizer.Secret(), identityID)
	if err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"preauth_token": preauth, "memberships": memberships})
}

// selectMembership finishes a login that resolveLogin found ambiguous:
// the preauth token proves the password/social check already happened for
// this identity, so this just confirms membershipID really belongs to it
// and mints the real tokens — the same "mint a token for a different
// context, from a proof already established" shape as impersonateTenant.
func (s *server) selectMembership(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PreauthToken string `json:"preauth_token"`
		MembershipID string `json:"membership_id"`
	}
	if !decode(w, r, &body) {
		return
	}
	identityID, err := auth.VerifyState(s.cfg.Tokenizer.Secret(), body.PreauthToken, 5*time.Minute)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid_token", "sessão de escolha expirada, faça login novamente")
		return
	}
	user, err := s.store.GetMembershipForIdentity(r.Context(), identityID, body.MembershipID)
	if err != nil {
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

	// Identity-scoped: managers never have a phone on file (they only
	// authenticate by e-mail), so a phone match here is always a client or
	// professional in practice — no role check needed anymore. The new
	// password applies to every barbershop this identity is enrolled in.
	identity, err := s.store.FindIdentityByPhone(r.Context(), phone)
	if err == nil {
		if password, hashErr := auth.GenerateTempPassword(); hashErr == nil {
			if hash, hashErr := auth.HashPassword(password); hashErr == nil {
				if setErr := s.store.SetPassword(r.Context(), identity.ID, hash); setErr == nil {
					if sendErr := s.cfg.Notifier.SendPassword(r.Context(), phone, password, channel); sendErr != nil {
						log.Printf("notify: failed to send recovery password: %v", sendErr)
					}
				}
			}
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		log.Printf("recover phone lookup: %v", err)
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Se o número estiver cadastrado, uma nova senha foi enviada."})
}

// grantCustomerAccess is how a barber/gestor gives a customer without
// e-mail a phone login. See grantAccess for the three cases it resolves to.
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
	s.grantAccess(w, r, claims.TenantID, domain.RoleClient, customer.Name, phone, "", customerID, channel)
}

// grantProfessionalAccess mirrors grantCustomerAccess, just for a
// professional's own login instead of a client's. Manager-only: unlike
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
	s.grantAccess(w, r, claims.TenantID, domain.RoleProfessional, professional.Name, phone, professionalID, "", channel)
}

// grantAccess is the shared resolution behind grantCustomerAccess and
// grantProfessionalAccess — exactly one of professionalID/customerID is
// set depending on the caller. Three cases:
//  1. This exact customer/professional already has a membership (staff
//     clicking the button again to recover a forgotten/locked password):
//     regenerate and resend the password, same as before the
//     identity/membership split.
//  2. The phone belongs to an identity that already has access somewhere
//     else (this same person already uses another barbershop): attach a
//     membership here without touching the password — it's the same
//     password everywhere, so there's nothing to regenerate or resend.
//  3. Nobody has ever seen this phone before: create the identity, the
//     membership, and a fresh password together.
func (s *server) grantAccess(w http.ResponseWriter, r *http.Request, tenantID, role, name, phone, professionalID, customerID, channel string) {
	existing, err := s.findExistingMembership(r.Context(), professionalID, customerID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		handleError(w, err)
		return
	}
	if err == nil {
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
		if err := s.store.SetPassword(r.Context(), existing.IdentityID, hash); err != nil {
			handleError(w, err)
			return
		}
		if err := s.cfg.Notifier.SendPassword(r.Context(), phone, password, channel); err != nil {
			handleError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Senha enviada com sucesso."})
		return
	}

	identity, err := s.store.FindIdentityByPhone(r.Context(), phone)
	if err == nil {
		if _, err := s.store.AttachMembership(r.Context(), identity.ID, tenantID, role, professionalID, customerID); err != nil {
			handleError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Essa pessoa já tinha acesso em outra barbearia — vínculo criado, a senha continua a mesma de sempre."})
		return
	}
	if !errors.Is(err, domain.ErrNotFound) {
		handleError(w, err)
		return
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
	if _, err := s.store.CreateIdentityWithMembership(r.Context(), name, "", phone, hash, "", "", tenantID, role, professionalID, customerID); err != nil {
		handleError(w, err)
		return
	}
	if err := s.cfg.Notifier.SendPassword(r.Context(), phone, password, channel); err != nil {
		handleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Senha enviada com sucesso."})
}

func (s *server) findExistingMembership(ctx context.Context, professionalID, customerID string) (domain.User, error) {
	if professionalID != "" {
		return s.store.FindMembershipByProfessionalID(ctx, professionalID)
	}
	return s.store.FindMembershipByCustomerID(ctx, customerID)
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
	user, err := s.store.GetMembershipByID(r.Context(), claims.UserID)
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
	user, err := s.store.GetMembershipByID(r.Context(), claims.UserID)
	respond(w, user, err)
}

func (s *server) listTenantsAdmin(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListTenants(r.Context())
	respond(w, items, err)
}

// adminSearchCustomers lets a superadmin find a customer by name/phone/
// e-mail across every barbershop, without impersonating tenant by tenant
// first. An empty query returns no rows rather than dumping every customer
// on the platform.
func (s *server) adminSearchCustomers(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusOK, []domain.AdminCustomerMatch{})
		return
	}
	items, err := s.store.SearchCustomersGlobal(r.Context(), query)
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
		user, options, err := s.findOrCreateSocialUser(r.Context(), providerName, tenantID, info)
		if err != nil {
			log.Printf("oauth upsert user (%s): %v", providerName, err)
			s.redirectWithError(w, r, "não foi possível concluir o login")
			return
		}
		if len(options) > 0 {
			s.redirectWithMembershipChoice(w, r, options)
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

// redirectWithMembershipChoice is oauthCallback's equivalent of
// respondMembershipChoice for the password-login path: same identity, more
// than one barbershop, so the frontend needs to ask which one before a
// real token can be minted (see selectMembership).
func (s *server) redirectWithMembershipChoice(w http.ResponseWriter, r *http.Request, options []domain.MembershipOption) {
	preauth, err := auth.SignState(s.cfg.Tokenizer.Secret(), options[0].IdentityID)
	if err != nil {
		s.redirectWithError(w, r, "não foi possível concluir o login")
		return
	}
	encoded, err := json.Marshal(options)
	if err != nil {
		s.redirectWithError(w, r, "não foi possível concluir o login")
		return
	}
	target := fmt.Sprintf("%s/#preauth_token=%s&memberships=%s",
		strings.TrimSuffix(s.cfg.FrontendURL, "/"), url.QueryEscape(preauth), url.QueryEscape(string(encoded)))
	http.Redirect(w, r, target, http.StatusFound)
}

// findOrCreateSocialUser links a Google/Facebook account to an existing
// identity matched by provider id, then by e-mail (this is how a manager
// or professional recovers access without a "forgot password" flow: sign
// in with the same e-mail via Google/Facebook). When tenantID is given
// (resolved from ?tenant=<slug> at oauthStart) and the identity has no
// membership there yet, attaches one as a new client instead of silently
// resolving to whichever other barbershop the identity already belonged
// to — that's what lets a client already known at barbershop A visit
// barbershop B's self-registration link for the first time and land in
// the right place. Without a tenant hint (staff recovering access via the
// same e-mail), falls back to the same "one active membership resolves
// directly, more than one needs a choice" rule password login uses — the
// third return value carries that choice, mirroring resolveLogin. If no
// identity matches at all and there's no tenant to enroll into,
// registration is rejected: there's no barbershop to join.
func (s *server) findOrCreateSocialUser(ctx context.Context, providerName, tenantID string, info auth.OAuthUserInfo) (domain.User, []domain.MembershipOption, error) {
	lookup, link := s.store.FindIdentityByGoogleID, s.store.LinkGoogleID
	if providerName == "facebook" {
		lookup, link = s.store.FindIdentityByFacebookID, s.store.LinkFacebookID
	}

	identity, err := lookup(ctx, info.ProviderID)
	if err != nil && !errors.Is(err, domain.ErrNotFound) {
		return domain.User{}, nil, err
	}
	if errors.Is(err, domain.ErrNotFound) {
		identity, err = s.store.FindIdentityByEmail(ctx, info.Email)
		if err != nil && !errors.Is(err, domain.ErrNotFound) {
			return domain.User{}, nil, err
		}
		if errors.Is(err, domain.ErrNotFound) {
			if tenantID == "" {
				return domain.User{}, nil, fmt.Errorf("cadastro social exige a barbearia (?tenant=<slug>)")
			}
			customer, err := s.store.CreateCustomer(ctx, tenantID, domain.Customer{Name: info.Name, Email: info.Email})
			if err != nil {
				return domain.User{}, nil, err
			}
			googleID, facebookID := "", ""
			if providerName == "google" {
				googleID = info.ProviderID
			} else {
				facebookID = info.ProviderID
			}
			user, err := s.store.CreateIdentityWithMembership(ctx, info.Name, info.Email, "", "", googleID, facebookID, tenantID, domain.RoleClient, "", customer.ID)
			return user, nil, err
		}
		if err := link(ctx, identity.ID, info.ProviderID); err != nil {
			return domain.User{}, nil, err
		}
	}

	if tenantID != "" {
		user, err := s.store.GetMembershipForIdentityAndTenant(ctx, identity.ID, tenantID)
		if err == nil {
			return user, nil, nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return domain.User{}, nil, err
		}
		customer, err := s.store.CreateCustomer(ctx, tenantID, domain.Customer{Name: identity.Name, Email: identity.Email, Phone: identity.Phone})
		if err != nil {
			return domain.User{}, nil, err
		}
		user, err = s.store.AttachMembership(ctx, identity.ID, tenantID, domain.RoleClient, "", customer.ID)
		return user, nil, err
	}

	memberships, err := s.store.ListMembershipsByIdentity(ctx, identity.ID)
	if err != nil {
		return domain.User{}, nil, err
	}
	if len(memberships) == 0 {
		return domain.User{}, nil, domain.ErrNotFound
	}
	if len(memberships) == 1 {
		user, err := s.store.GetMembershipByID(ctx, memberships[0].MembershipID)
		return user, nil, err
	}
	return domain.User{}, memberships, nil
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
func (s *server) updateService(w http.ResponseWriter, r *http.Request) {
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
	updated, err := s.store.UpdateService(r.Context(), claims.TenantID, r.PathValue("id"), item)
	respond(w, updated, err)
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
func (s *server) updateProfessional(w http.ResponseWriter, r *http.Request) {
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
	updated, err := s.store.UpdateProfessional(r.Context(), claims.TenantID, r.PathValue("id"), item)
	respond(w, updated, err)
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
func (s *server) updateCustomer(w http.ResponseWriter, r *http.Request) {
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
	updated, err := s.store.UpdateCustomer(r.Context(), claims.TenantID, r.PathValue("id"), item)
	respond(w, updated, err)
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

// getReport defaults to the current calendar month, unlike
// listAppointments (which defaults to today) - a report is normally read
// over a longer window than the daily agenda.
func (s *server) getReport(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	from, err := parseTime(r.URL.Query().Get("from"), time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()))
	if err != nil {
		writeError(w, 400, "validation_error", "from inválido")
		return
	}
	to, err := parseTime(r.URL.Query().Get("to"), from.AddDate(0, 1, 0))
	if err != nil || !to.After(from) {
		writeError(w, 400, "validation_error", "to inválido")
		return
	}
	claims, _ := claimsFromContext(r)
	report, err := s.store.GetReport(r.Context(), claims.TenantID, from, to)
	respond(w, report, err)
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
