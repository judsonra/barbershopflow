package http

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/example/barberflow/backend/internal/domain"
)

// fakeStore is an in-memory implementation of Store shared by every handler
// test in this package. Tests configure it directly (seed maps, set errs)
// before exercising a handler through the real server built by New — no
// database, no mocking framework.
//
// Data that is genuinely per-tenant in the real schema (services,
// professionals, customers, appointments, holidays, schedules, time-off,
// professional-services) is kept in maps nested by tenantID, so tests can
// catch a handler accidentally leaking data across tenants. Identities and
// memberships are global, matching the real identity/membership split.
type fakeStore struct {
	services      map[string]map[string]domain.Service
	professionals map[string]map[string]domain.Professional
	customers     map[string]map[string]domain.Customer
	appointments  map[string]map[string]domain.Appointment
	holidays      map[string]map[string]domain.Holiday
	schedules     map[string][]domain.ScheduleEntry
	timeOff       map[string]map[string]domain.TimeOff
	profServices  map[string][]domain.ProfessionalService

	identities  map[string]domain.Identity
	memberships map[string]domain.User
	tenants     map[string]domain.Tenant

	membershipOptions map[string][]domain.MembershipOption // by identityID
	audits            []domain.ImpersonationAudit

	seq int

	// errs lets a test inject an error for a specific Store method by name,
	// e.g. store.errs["CreateAppointment"] = domain.ErrScheduleConflict.
	// Checked before the method's normal in-memory logic runs, so a test
	// doesn't need to fight the fake's bookkeeping to exercise an error path.
	errs map[string]error

	// incrementFailedLoginFn overrides the default IncrementFailedLogin
	// behavior when a test needs call-by-call control (e.g. asserting the
	// exact attempt at which an account locks). Falls back to a real
	// counter against domain.MaxLoginAttempts when nil.
	incrementFailedLoginFn func(identityID string) (bool, error)
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		services:          map[string]map[string]domain.Service{},
		professionals:     map[string]map[string]domain.Professional{},
		customers:         map[string]map[string]domain.Customer{},
		appointments:      map[string]map[string]domain.Appointment{},
		holidays:          map[string]map[string]domain.Holiday{},
		schedules:         map[string][]domain.ScheduleEntry{},
		timeOff:           map[string]map[string]domain.TimeOff{},
		profServices:      map[string][]domain.ProfessionalService{},
		identities:        map[string]domain.Identity{},
		memberships:       map[string]domain.User{},
		tenants:           map[string]domain.Tenant{},
		membershipOptions: map[string][]domain.MembershipOption{},
		errs:              map[string]error{},
	}
}

func (s *fakeStore) err(method string) error { return s.errs[method] }

func (s *fakeStore) newID(prefix string) string {
	s.seq++
	return fmt.Sprintf("%s-%d", prefix, s.seq)
}

func scopeKey(tenantID, professionalID string) string { return tenantID + "/" + professionalID }

// --- services ---

func (s *fakeStore) ListServices(_ context.Context, tenantID string) ([]domain.Service, error) {
	if err := s.err("ListServices"); err != nil {
		return nil, err
	}
	out := make([]domain.Service, 0, len(s.services[tenantID]))
	for _, v := range s.services[tenantID] {
		out = append(out, v)
	}
	return out, nil
}

func (s *fakeStore) CreateService(_ context.Context, tenantID string, item domain.Service) (domain.Service, error) {
	if err := s.err("CreateService"); err != nil {
		return domain.Service{}, err
	}
	item.ID = s.newID("service")
	item.Active = true
	if s.services[tenantID] == nil {
		s.services[tenantID] = map[string]domain.Service{}
	}
	s.services[tenantID][item.ID] = item
	return item, nil
}

func (s *fakeStore) UpdateService(_ context.Context, tenantID, id string, item domain.Service) (domain.Service, error) {
	if err := s.err("UpdateService"); err != nil {
		return domain.Service{}, err
	}
	if _, ok := s.services[tenantID][id]; !ok {
		return domain.Service{}, domain.ErrNotFound
	}
	item.ID = id
	s.services[tenantID][id] = item
	return item, nil
}

// --- professionals ---

func (s *fakeStore) ListProfessionals(_ context.Context, tenantID string) ([]domain.Professional, error) {
	if err := s.err("ListProfessionals"); err != nil {
		return nil, err
	}
	out := make([]domain.Professional, 0, len(s.professionals[tenantID]))
	for _, v := range s.professionals[tenantID] {
		out = append(out, v)
	}
	return out, nil
}

func (s *fakeStore) CreateProfessional(_ context.Context, tenantID string, item domain.Professional) (domain.Professional, error) {
	if err := s.err("CreateProfessional"); err != nil {
		return domain.Professional{}, err
	}
	item.ID = s.newID("prof")
	item.Active = true
	if s.professionals[tenantID] == nil {
		s.professionals[tenantID] = map[string]domain.Professional{}
	}
	s.professionals[tenantID][item.ID] = item
	return item, nil
}

func (s *fakeStore) UpdateProfessional(_ context.Context, tenantID, id string, item domain.Professional) (domain.Professional, error) {
	if err := s.err("UpdateProfessional"); err != nil {
		return domain.Professional{}, err
	}
	if _, ok := s.professionals[tenantID][id]; !ok {
		return domain.Professional{}, domain.ErrNotFound
	}
	item.ID = id
	s.professionals[tenantID][id] = item
	return item, nil
}

func (s *fakeStore) GetProfessionalByID(_ context.Context, tenantID, id string) (domain.Professional, error) {
	if err := s.err("GetProfessionalByID"); err != nil {
		return domain.Professional{}, err
	}
	p, ok := s.professionals[tenantID][id]
	if !ok {
		return domain.Professional{}, domain.ErrNotFound
	}
	return p, nil
}

func (s *fakeStore) UpdateProfessionalPhone(_ context.Context, tenantID, professionalID, phone string) error {
	if err := s.err("UpdateProfessionalPhone"); err != nil {
		return err
	}
	p, ok := s.professionals[tenantID][professionalID]
	if !ok {
		return domain.ErrNotFound
	}
	p.Phone = phone
	s.professionals[tenantID][professionalID] = p
	return nil
}

// --- customers ---

func (s *fakeStore) ListCustomers(_ context.Context, tenantID string, page, limit int) (domain.CustomerPage, error) {
	if err := s.err("ListCustomers"); err != nil {
		return domain.CustomerPage{}, err
	}
	out := make([]domain.Customer, 0, len(s.customers[tenantID]))
	for _, v := range s.customers[tenantID] {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	total := len(out)
	if limit <= 0 {
		return domain.CustomerPage{Items: out, Total: total}, nil
	}
	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return domain.CustomerPage{Items: out[start:end], Total: total}, nil
}

func (s *fakeStore) SearchCustomersGlobal(_ context.Context, query string) ([]domain.AdminCustomerMatch, error) {
	if err := s.err("SearchCustomersGlobal"); err != nil {
		return nil, err
	}
	query = strings.ToLower(query)
	out := []domain.AdminCustomerMatch{}
	for tenantID, byID := range s.customers {
		tenant := s.tenants[tenantID]
		for _, c := range byID {
			if strings.Contains(strings.ToLower(c.Name), query) ||
				strings.Contains(strings.ToLower(c.Phone), query) ||
				strings.Contains(strings.ToLower(c.Email), query) {
				out = append(out, domain.AdminCustomerMatch{
					Customer: c, TenantID: tenantID, TenantName: tenant.Name, TenantSlug: tenant.Slug,
				})
			}
		}
	}
	return out, nil
}

func (s *fakeStore) CreateCustomer(_ context.Context, tenantID string, item domain.Customer) (domain.Customer, error) {
	if err := s.err("CreateCustomer"); err != nil {
		return domain.Customer{}, err
	}
	item.ID = s.newID("customer")
	item.Active = true
	if s.customers[tenantID] == nil {
		s.customers[tenantID] = map[string]domain.Customer{}
	}
	s.customers[tenantID][item.ID] = item
	return item, nil
}

func (s *fakeStore) UpdateCustomer(_ context.Context, tenantID, id string, item domain.Customer) (domain.Customer, error) {
	if err := s.err("UpdateCustomer"); err != nil {
		return domain.Customer{}, err
	}
	if _, ok := s.customers[tenantID][id]; !ok {
		return domain.Customer{}, domain.ErrNotFound
	}
	item.ID = id
	s.customers[tenantID][id] = item
	return item, nil
}

func (s *fakeStore) GetCustomerByID(_ context.Context, tenantID, id string) (domain.Customer, error) {
	if err := s.err("GetCustomerByID"); err != nil {
		return domain.Customer{}, err
	}
	c, ok := s.customers[tenantID][id]
	if !ok {
		return domain.Customer{}, domain.ErrNotFound
	}
	return c, nil
}

func (s *fakeStore) UpdateCustomerPhone(_ context.Context, tenantID, customerID, phone string) error {
	if err := s.err("UpdateCustomerPhone"); err != nil {
		return err
	}
	c, ok := s.customers[tenantID][customerID]
	if !ok {
		return domain.ErrNotFound
	}
	c.Phone = phone
	s.customers[tenantID][customerID] = c
	return nil
}

// --- impersonation / audit ---

func (s *fakeStore) RecordImpersonation(_ context.Context, actorMembershipID, tenantID string) error {
	if err := s.err("RecordImpersonation"); err != nil {
		return err
	}
	actor := s.memberships[actorMembershipID]
	tenant := s.tenants[tenantID]
	s.audits = append(s.audits, domain.ImpersonationAudit{
		ID: s.newID("audit"), ActorName: actor.Name, ActorEmail: actor.Email,
		TenantID: tenantID, TenantName: tenant.Name, TenantSlug: tenant.Slug, CreatedAt: time.Now(),
	})
	return nil
}

func (s *fakeStore) ListImpersonationAudit(_ context.Context) ([]domain.ImpersonationAudit, error) {
	if err := s.err("ListImpersonationAudit"); err != nil {
		return nil, err
	}
	return s.audits, nil
}

// --- appointments ---

func (s *fakeStore) ListAppointments(_ context.Context, tenantID, customerID, professionalID string, from, to time.Time) ([]domain.Appointment, error) {
	if err := s.err("ListAppointments"); err != nil {
		return nil, err
	}
	out := []domain.Appointment{}
	for _, a := range s.appointments[tenantID] {
		if customerID != "" && a.CustomerID != customerID {
			continue
		}
		if professionalID != "" && a.ProfessionalID != professionalID {
			continue
		}
		if a.StartsAt.Before(from) || !a.StartsAt.Before(to) {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

func (s *fakeStore) GetReport(_ context.Context, tenantID string, from, to time.Time) (domain.Report, error) {
	if err := s.err("GetReport"); err != nil {
		return domain.Report{}, err
	}
	return domain.Report{From: from, To: to}, nil
}

func (s *fakeStore) CreateAppointment(_ context.Context, tenantID, status string, item domain.Appointment) (domain.Appointment, error) {
	if err := s.err("CreateAppointment"); err != nil {
		return domain.Appointment{}, err
	}
	item.ID = s.newID("appt")
	item.Status = status
	item.CreatedAt = time.Now()
	if s.appointments[tenantID] == nil {
		s.appointments[tenantID] = map[string]domain.Appointment{}
	}
	s.appointments[tenantID][item.ID] = item
	return item, nil
}

func (s *fakeStore) UpdateAppointmentStatus(_ context.Context, tenantID, id, status string) (domain.Appointment, error) {
	if err := s.err("UpdateAppointmentStatus"); err != nil {
		return domain.Appointment{}, err
	}
	a, ok := s.appointments[tenantID][id]
	if !ok {
		return domain.Appointment{}, domain.ErrNotFound
	}
	a.Status = status
	s.appointments[tenantID][id] = a
	return a, nil
}

func (s *fakeStore) GetAppointmentProfessionalID(_ context.Context, tenantID, id string) (string, error) {
	if err := s.err("GetAppointmentProfessionalID"); err != nil {
		return "", err
	}
	a, ok := s.appointments[tenantID][id]
	if !ok {
		return "", domain.ErrNotFound
	}
	return a.ProfessionalID, nil
}

func (s *fakeStore) GetAppointmentForCancellation(_ context.Context, tenantID, id string) (string, time.Time, error) {
	if err := s.err("GetAppointmentForCancellation"); err != nil {
		return "", time.Time{}, err
	}
	a, ok := s.appointments[tenantID][id]
	if !ok {
		return "", time.Time{}, domain.ErrNotFound
	}
	return a.CustomerID, a.StartsAt, nil
}

// --- identities ---

func (s *fakeStore) FindIdentityByEmail(_ context.Context, email string) (domain.Identity, error) {
	if err := s.err("FindIdentityByEmail"); err != nil {
		return domain.Identity{}, err
	}
	for _, id := range s.identities {
		if id.Email != "" && id.Email == email {
			return id, nil
		}
	}
	return domain.Identity{}, domain.ErrNotFound
}

func (s *fakeStore) FindIdentityByPhone(_ context.Context, phone string) (domain.Identity, error) {
	if err := s.err("FindIdentityByPhone"); err != nil {
		return domain.Identity{}, err
	}
	for _, id := range s.identities {
		if id.Phone != "" && id.Phone == phone {
			return id, nil
		}
	}
	return domain.Identity{}, domain.ErrNotFound
}

func (s *fakeStore) FindIdentityByGoogleID(_ context.Context, googleID string) (domain.Identity, error) {
	if err := s.err("FindIdentityByGoogleID"); err != nil {
		return domain.Identity{}, err
	}
	for _, id := range s.identities {
		if id.GoogleID != "" && id.GoogleID == googleID {
			return id, nil
		}
	}
	return domain.Identity{}, domain.ErrNotFound
}

func (s *fakeStore) FindIdentityByFacebookID(_ context.Context, facebookID string) (domain.Identity, error) {
	if err := s.err("FindIdentityByFacebookID"); err != nil {
		return domain.Identity{}, err
	}
	for _, id := range s.identities {
		if id.FacebookID != "" && id.FacebookID == facebookID {
			return id, nil
		}
	}
	return domain.Identity{}, domain.ErrNotFound
}

func (s *fakeStore) LinkGoogleID(_ context.Context, identityID, googleID string) error {
	if err := s.err("LinkGoogleID"); err != nil {
		return err
	}
	id, ok := s.identities[identityID]
	if !ok {
		return domain.ErrNotFound
	}
	id.GoogleID = googleID
	s.identities[identityID] = id
	return nil
}

func (s *fakeStore) LinkFacebookID(_ context.Context, identityID, facebookID string) error {
	if err := s.err("LinkFacebookID"); err != nil {
		return err
	}
	id, ok := s.identities[identityID]
	if !ok {
		return domain.ErrNotFound
	}
	id.FacebookID = facebookID
	s.identities[identityID] = id
	return nil
}

func (s *fakeStore) SetPassword(_ context.Context, identityID, passwordHash string) error {
	if err := s.err("SetPassword"); err != nil {
		return err
	}
	id, ok := s.identities[identityID]
	if !ok {
		return domain.ErrNotFound
	}
	id.PasswordHash = passwordHash
	s.identities[identityID] = id
	return nil
}

func (s *fakeStore) ResetLoginAttempts(_ context.Context, identityID string) error {
	if err := s.err("ResetLoginAttempts"); err != nil {
		return err
	}
	id, ok := s.identities[identityID]
	if !ok {
		return domain.ErrNotFound
	}
	id.FailedLoginAttempts = 0
	id.LockedAt = time.Time{}
	s.identities[identityID] = id
	return nil
}

func (s *fakeStore) IncrementFailedLogin(_ context.Context, identityID string) (bool, error) {
	if s.incrementFailedLoginFn != nil {
		return s.incrementFailedLoginFn(identityID)
	}
	if err := s.err("IncrementFailedLogin"); err != nil {
		return false, err
	}
	id, ok := s.identities[identityID]
	if !ok {
		return false, domain.ErrNotFound
	}
	id.FailedLoginAttempts++
	locked := id.FailedLoginAttempts >= domain.MaxLoginAttempts
	if locked {
		id.LockedAt = time.Now()
	}
	s.identities[identityID] = id
	return locked, nil
}

// --- memberships ---

func (s *fakeStore) GetMembershipByID(_ context.Context, membershipID string) (domain.User, error) {
	if err := s.err("GetMembershipByID"); err != nil {
		return domain.User{}, err
	}
	u, ok := s.memberships[membershipID]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (s *fakeStore) GetMembershipForIdentityAndTenant(_ context.Context, identityID, tenantID string) (domain.User, error) {
	if err := s.err("GetMembershipForIdentityAndTenant"); err != nil {
		return domain.User{}, err
	}
	for _, opt := range s.membershipOptions[identityID] {
		if opt.TenantID == tenantID {
			return s.memberships[opt.MembershipID], nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

func (s *fakeStore) GetMembershipForIdentity(_ context.Context, identityID, membershipID string) (domain.User, error) {
	if err := s.err("GetMembershipForIdentity"); err != nil {
		return domain.User{}, err
	}
	for _, opt := range s.membershipOptions[identityID] {
		if opt.MembershipID == membershipID {
			return s.memberships[membershipID], nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

func (s *fakeStore) FindMembershipByCustomerID(_ context.Context, customerID string) (domain.User, error) {
	if err := s.err("FindMembershipByCustomerID"); err != nil {
		return domain.User{}, err
	}
	for _, u := range s.memberships {
		if u.CustomerID == customerID {
			return u, nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

func (s *fakeStore) FindMembershipByProfessionalID(_ context.Context, professionalID string) (domain.User, error) {
	if err := s.err("FindMembershipByProfessionalID"); err != nil {
		return domain.User{}, err
	}
	for _, u := range s.memberships {
		if u.ProfessionalID == professionalID {
			return u, nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

func (s *fakeStore) ListMembershipsByIdentity(_ context.Context, identityID string) ([]domain.MembershipOption, error) {
	if err := s.err("ListMembershipsByIdentity"); err != nil {
		return nil, err
	}
	return s.membershipOptions[identityID], nil
}

func (s *fakeStore) AttachMembership(_ context.Context, identityID, tenantID, role, professionalID, customerID string) (domain.User, error) {
	if err := s.err("AttachMembership"); err != nil {
		return domain.User{}, err
	}
	identity := s.identities[identityID]
	tenant := s.tenants[tenantID]
	membershipID := s.newID("membership")
	user := domain.User{
		ID: membershipID, IdentityID: identityID, TenantID: tenantID, Name: identity.Name,
		Email: identity.Email, Phone: identity.Phone, PasswordHash: identity.PasswordHash,
		Role: role, ProfessionalID: professionalID, CustomerID: customerID, Active: true, CreatedAt: time.Now(),
	}
	s.memberships[membershipID] = user
	s.membershipOptions[identityID] = append(s.membershipOptions[identityID], domain.MembershipOption{
		MembershipID: membershipID, IdentityID: identityID, TenantID: tenantID,
		TenantName: tenant.Name, TenantSlug: tenant.Slug, Role: role,
	})
	return user, nil
}

func (s *fakeStore) SetMembershipRole(_ context.Context, membershipID, role string) error {
	if err := s.err("SetMembershipRole"); err != nil {
		return err
	}
	u, ok := s.memberships[membershipID]
	if !ok {
		return domain.ErrNotFound
	}
	u.Role = role
	s.memberships[membershipID] = u
	for identityID, opts := range s.membershipOptions {
		for i, opt := range opts {
			if opt.MembershipID == membershipID {
				s.membershipOptions[identityID][i].Role = role
			}
		}
	}
	return nil
}

func (s *fakeStore) CreateIdentityWithMembership(_ context.Context, name, email, phone, passwordHash, googleID, facebookID, tenantID, role, professionalID, customerID string) (domain.User, error) {
	if err := s.err("CreateIdentityWithMembership"); err != nil {
		return domain.User{}, err
	}
	identityID := s.newID("identity")
	s.identities[identityID] = domain.Identity{
		ID: identityID, Name: name, Email: email, Phone: phone, PasswordHash: passwordHash,
		GoogleID: googleID, FacebookID: facebookID, CreatedAt: time.Now(),
	}
	tenant := s.tenants[tenantID]
	membershipID := s.newID("membership")
	user := domain.User{
		ID: membershipID, IdentityID: identityID, TenantID: tenantID, Name: name, Email: email,
		Phone: phone, PasswordHash: passwordHash, GoogleID: googleID, FacebookID: facebookID,
		Role: role, ProfessionalID: professionalID, CustomerID: customerID, Active: true, CreatedAt: time.Now(),
	}
	s.memberships[membershipID] = user
	s.membershipOptions[identityID] = append(s.membershipOptions[identityID], domain.MembershipOption{
		MembershipID: membershipID, IdentityID: identityID, TenantID: tenantID,
		TenantName: tenant.Name, TenantSlug: tenant.Slug, Role: role,
	})
	return user, nil
}

// --- tenants ---

func (s *fakeStore) GetTenantBySlug(_ context.Context, slug string) (domain.Tenant, error) {
	if err := s.err("GetTenantBySlug"); err != nil {
		return domain.Tenant{}, err
	}
	for _, t := range s.tenants {
		if t.Slug == slug {
			return t, nil
		}
	}
	return domain.Tenant{}, domain.ErrNotFound
}

func (s *fakeStore) TenantNameAvailable(_ context.Context, name string) (bool, error) {
	if err := s.err("TenantNameAvailable"); err != nil {
		return false, err
	}
	for _, t := range s.tenants {
		if strings.EqualFold(t.Name, name) {
			return false, nil
		}
	}
	return true, nil
}

func (s *fakeStore) ListTenants(_ context.Context) ([]domain.Tenant, error) {
	if err := s.err("ListTenants"); err != nil {
		return nil, err
	}
	out := make([]domain.Tenant, 0, len(s.tenants))
	for _, t := range s.tenants {
		out = append(out, t)
	}
	return out, nil
}

func (s *fakeStore) GetFirstManagerByTenant(_ context.Context, tenantID string) (domain.User, error) {
	if err := s.err("GetFirstManagerByTenant"); err != nil {
		return domain.User{}, err
	}
	for _, u := range s.memberships {
		if u.TenantID == tenantID && u.Role == domain.RoleManager {
			return u, nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

func (s *fakeStore) GetTenantByID(_ context.Context, id string) (domain.Tenant, error) {
	if err := s.err("GetTenantByID"); err != nil {
		return domain.Tenant{}, err
	}
	t, ok := s.tenants[id]
	if !ok {
		return domain.Tenant{}, domain.ErrNotFound
	}
	return t, nil
}

func (s *fakeStore) UpdateTenant(_ context.Context, tenantID, name, slug string, selfSchedulingEnabled, autoConfirmAppointments bool, cancellationWindowHours int) (domain.Tenant, error) {
	if err := s.err("UpdateTenant"); err != nil {
		return domain.Tenant{}, err
	}
	t, ok := s.tenants[tenantID]
	if !ok {
		return domain.Tenant{}, domain.ErrNotFound
	}
	t.Name, t.Slug = name, slug
	t.SelfSchedulingEnabled, t.AutoConfirmAppointments = selfSchedulingEnabled, autoConfirmAppointments
	t.CancellationWindowHours = cancellationWindowHours
	s.tenants[tenantID] = t
	return t, nil
}

func (s *fakeStore) CreateTenantWithManager(_ context.Context, tenantName, slug, managerName, email, passwordHash string) (domain.Tenant, domain.User, error) {
	if err := s.err("CreateTenantWithManager"); err != nil {
		return domain.Tenant{}, domain.User{}, err
	}
	tenantID := s.newID("tenant")
	tenant := domain.Tenant{ID: tenantID, Name: tenantName, Slug: slug, Active: true, CreatedAt: time.Now()}
	s.tenants[tenantID] = tenant

	identityID := s.newID("identity")
	s.identities[identityID] = domain.Identity{ID: identityID, Name: managerName, Email: email, PasswordHash: passwordHash, CreatedAt: time.Now()}

	membershipID := s.newID("membership")
	manager := domain.User{
		ID: membershipID, IdentityID: identityID, TenantID: tenantID, Name: managerName, Email: email,
		PasswordHash: passwordHash, Role: domain.RoleManager, Active: true, CreatedAt: time.Now(),
	}
	s.memberships[membershipID] = manager
	s.membershipOptions[identityID] = append(s.membershipOptions[identityID], domain.MembershipOption{
		MembershipID: membershipID, IdentityID: identityID, TenantID: tenantID,
		TenantName: tenantName, TenantSlug: slug, Role: domain.RoleManager,
	})
	return tenant, manager, nil
}

// --- holidays ---

func (s *fakeStore) ListHolidays(_ context.Context, tenantID string) ([]domain.Holiday, error) {
	if err := s.err("ListHolidays"); err != nil {
		return nil, err
	}
	out := make([]domain.Holiday, 0, len(s.holidays[tenantID]))
	for _, h := range s.holidays[tenantID] {
		out = append(out, h)
	}
	return out, nil
}

func (s *fakeStore) CreateHoliday(_ context.Context, tenantID string, item domain.Holiday) (domain.Holiday, error) {
	if err := s.err("CreateHoliday"); err != nil {
		return domain.Holiday{}, err
	}
	item.ID = s.newID("holiday")
	item.CreatedAt = time.Now()
	if s.holidays[tenantID] == nil {
		s.holidays[tenantID] = map[string]domain.Holiday{}
	}
	s.holidays[tenantID][item.ID] = item
	return item, nil
}

func (s *fakeStore) DeleteHoliday(_ context.Context, tenantID, id string) error {
	if err := s.err("DeleteHoliday"); err != nil {
		return err
	}
	if _, ok := s.holidays[tenantID][id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.holidays[tenantID], id)
	return nil
}

// --- professional schedule / time-off / services ---

func (s *fakeStore) GetProfessionalSchedule(_ context.Context, tenantID, professionalID string) ([]domain.ScheduleEntry, error) {
	if err := s.err("GetProfessionalSchedule"); err != nil {
		return nil, err
	}
	return s.schedules[scopeKey(tenantID, professionalID)], nil
}

func (s *fakeStore) SetProfessionalSchedule(_ context.Context, tenantID, professionalID string, entries []domain.ScheduleEntry) ([]domain.ScheduleEntry, error) {
	if err := s.err("SetProfessionalSchedule"); err != nil {
		return nil, err
	}
	s.schedules[scopeKey(tenantID, professionalID)] = entries
	return entries, nil
}

func (s *fakeStore) ListTimeOff(_ context.Context, tenantID, professionalID string, from, to time.Time) ([]domain.TimeOff, error) {
	if err := s.err("ListTimeOff"); err != nil {
		return nil, err
	}
	out := []domain.TimeOff{}
	for _, t := range s.timeOff[scopeKey(tenantID, professionalID)] {
		if t.EndsAt.Before(from) || t.StartsAt.After(to) {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func (s *fakeStore) CreateTimeOff(_ context.Context, tenantID, professionalID string, item domain.TimeOff) (domain.TimeOff, error) {
	if err := s.err("CreateTimeOff"); err != nil {
		return domain.TimeOff{}, err
	}
	item.ID = s.newID("timeoff")
	item.ProfessionalID = professionalID
	item.CreatedAt = time.Now()
	key := scopeKey(tenantID, professionalID)
	if s.timeOff[key] == nil {
		s.timeOff[key] = map[string]domain.TimeOff{}
	}
	s.timeOff[key][item.ID] = item
	return item, nil
}

func (s *fakeStore) DeleteTimeOff(_ context.Context, tenantID, professionalID, id string) error {
	if err := s.err("DeleteTimeOff"); err != nil {
		return err
	}
	key := scopeKey(tenantID, professionalID)
	if _, ok := s.timeOff[key][id]; !ok {
		return domain.ErrNotFound
	}
	delete(s.timeOff[key], id)
	return nil
}

func (s *fakeStore) ListProfessionalServices(_ context.Context, tenantID, professionalID string) ([]domain.ProfessionalService, error) {
	if err := s.err("ListProfessionalServices"); err != nil {
		return nil, err
	}
	return s.profServices[scopeKey(tenantID, professionalID)], nil
}

func (s *fakeStore) SetProfessionalServices(_ context.Context, tenantID, professionalID string, entries []domain.ProfessionalService) ([]domain.ProfessionalService, error) {
	if err := s.err("SetProfessionalServices"); err != nil {
		return nil, err
	}
	s.profServices[scopeKey(tenantID, professionalID)] = entries
	return entries, nil
}
