#!/bin/sh
# Creates the IAMKit role + database inside the shared Postgres instance.
# Runs once, on first container initialization (empty data dir), via the
# official postgres image's /docker-entrypoint-initdb.d hook.
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = '${IAMKIT_DB_USER}') THEN
            CREATE ROLE "${IAMKIT_DB_USER}" LOGIN PASSWORD '${IAMKIT_DB_PASSWORD}';
        END IF;
    END
    \$\$;

    SELECT 'CREATE DATABASE "${IAMKIT_DB_NAME}" OWNER "${IAMKIT_DB_USER}"'
    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '${IAMKIT_DB_NAME}')\gexec
EOSQL
