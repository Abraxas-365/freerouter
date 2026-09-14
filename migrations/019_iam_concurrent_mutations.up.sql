-- Persistence uses version 0 exclusively for new entities, >=1 for loaded rows.
CREATE FUNCTION advance_iam_row_version() RETURNS trigger AS $$
BEGIN
 NEW.version := OLD.version + 1;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
ALTER TABLE users ADD COLUMN version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0);
CREATE TRIGGER advance_users_version BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION advance_iam_row_version();
ALTER TABLE api_keys ADD COLUMN version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0);
CREATE FUNCTION advance_api_key_security_version() RETURNS trigger AS $$
BEGIN
 IF (to_jsonb(NEW) - 'last_used_at' - 'version') IS DISTINCT FROM (to_jsonb(OLD) - 'last_used_at' - 'version') THEN
  NEW.version := OLD.version + 1;
 ELSE
  NEW.version := OLD.version;
 END IF;
 RETURN NEW;
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER advance_api_keys_version BEFORE UPDATE ON api_keys FOR EACH ROW EXECUTE FUNCTION advance_api_key_security_version();
ALTER TABLE roles ADD COLUMN version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0);
CREATE TRIGGER advance_roles_version BEFORE UPDATE ON roles FOR EACH ROW EXECUTE FUNCTION advance_iam_row_version();
ALTER TABLE invitations ADD COLUMN version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0);
CREATE TRIGGER advance_invitations_version BEFORE UPDATE ON invitations FOR EACH ROW EXECUTE FUNCTION advance_iam_row_version();
ALTER TABLE tenants ADD COLUMN version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0);
CREATE TRIGGER advance_tenants_version BEFORE UPDATE ON tenants FOR EACH ROW EXECUTE FUNCTION advance_iam_row_version();

-- Legacy pending role invitations must be reissued; their authority was not pinned.
ALTER TABLE invitations ADD COLUMN role_version BIGINT NOT NULL DEFAULT 0;
