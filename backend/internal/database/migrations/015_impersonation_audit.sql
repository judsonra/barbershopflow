CREATE TABLE impersonation_audits (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_membership_id uuid NOT NULL REFERENCES memberships(id),
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX impersonation_audits_created_at_idx ON impersonation_audits(created_at DESC);
