-- Remove subscription/trial columns and constraints from tenants

ALTER TABLE tenants DROP CONSTRAINT IF EXISTS chk_subscription_plan;
ALTER TABLE tenants DROP CONSTRAINT IF EXISTS chk_tenant_status;

DROP INDEX IF EXISTS idx_tenants_subscription_plan;

ALTER TABLE tenants DROP COLUMN IF EXISTS subscription_plan;
ALTER TABLE tenants DROP COLUMN IF EXISTS trial_expires_at;
ALTER TABLE tenants DROP COLUMN IF EXISTS subscription_expires_at;

-- Update status default and constraint (remove TRIAL status)
ALTER TABLE tenants ALTER COLUMN status SET DEFAULT 'ACTIVE';
ALTER TABLE tenants ADD CONSTRAINT chk_tenant_status CHECK (status IN ('ACTIVE', 'SUSPENDED', 'CANCELED'));

-- Migrate any existing TRIAL tenants to ACTIVE
UPDATE tenants SET status = 'ACTIVE' WHERE status = 'TRIAL';
