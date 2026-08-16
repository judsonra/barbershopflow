package repository

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/example/barberflow/backend/internal/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// testPool is shared by every integration test in this package: starting a
// container + running all migrations is the expensive part, so it happens
// once per `go test` run (in TestMain) instead of once per test. Each test
// is still independent — it creates its own tenant/fixture rows (via
// CreateTenantWithManager, unique names/slugs) rather than relying on
// state left by another test.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	// testing.Short() reads flags that TestMain must parse itself — see
	// https://pkg.go.dev/testing#hdr-Main — since TestMain replaces the
	// generated main that would normally do this.
	flag.Parse()
	if testing.Short() {
		os.Exit(m.Run())
	}

	ctx := context.Background()
	container, err := postgres.Run(ctx, "postgres:17-alpine",
		postgres.WithDatabase("barbershop_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "repository integration tests: start postgres container (is Docker running?): %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = container.Terminate(context.Background()) }()

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "repository integration tests: connection string: %v\n", err)
		os.Exit(1)
	}
	pool, err := database.Open(ctx, connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "repository integration tests: open pool: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := database.Migrate(ctx, pool); err != nil {
		fmt.Fprintf(os.Stderr, "repository integration tests: migrate: %v\n", err)
		os.Exit(1)
	}
	testPool = pool

	os.Exit(m.Run())
}

// newTestRepo skips the test under -short (no container was started) and
// otherwise returns a Repository backed by the shared migrated pool.
func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	if testing.Short() || testPool == nil {
		t.Skip("skipping integration test: requires Docker (run without -short)")
	}
	return New(testPool)
}

// newTestTenant creates a fresh tenant + manager for a test to scope its
// fixtures under, with a unique name/slug so parallel or repeated runs
// against the same shared Postgres never collide.
func newTestTenant(t *testing.T, repo *Repository) (tenantID string) {
	t.Helper()
	suffix := uniqueSuffix()
	tenant, _, err := repo.CreateTenantWithManager(context.Background(),
		"Barbearia Teste "+suffix, "barbearia-teste-"+suffix, "Gestor Teste", "gestor-"+suffix+"@example.com", "hash")
	if err != nil {
		t.Fatalf("create test tenant: %v", err)
	}
	return tenant.ID
}

var suffixCounter int

func uniqueSuffix() string {
	suffixCounter++
	return fmt.Sprintf("%d-%d", os.Getpid(), suffixCounter)
}
