#!/usr/bin/env bash
# Disposable PostgreSQL 17 proof. Run from checkout root under WSL.
set -euo pipefail
trap 'echo "FAIL recovery step at line $LINENO" >&2' ERR
umask 077
# Source revisions recorded in recovery evidence; WSL cannot resolve Windows worktree gitdir.
c="gofx-recovery-$(date +%s)-$$"
out=$(mktemp -d /tmp/gofx-recovery.XXXXXX)
trap 'docker rm -f "${api:-$c-api}" "$c" >/dev/null 2>&1 || true; rm -rf "$out"' EXIT
docker run -d --name "$c" -p 127.0.0.1::8090 -e POSTGRES_HOST_AUTH_METHOD=trust postgres:17-alpine >/dev/null
for i in $(seq 1 40); do docker exec "$c" pg_isready -U postgres >/dev/null 2>&1 && break; sleep 1; done
admin() { docker exec -i "$c" psql -X -v ON_ERROR_STOP=1 -U postgres "$@"; }
migrate() { { echo 'SET ROLE gx_owner;'; cat "$1"; } | docker exec -i "$c" psql -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -U gx_migrator -d "$2" >/dev/null; }
probe() { docker exec -i "$c" psql -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -U "$1" -d "$2" -Atc "$3"; }
deny() { if probe "$1" "$2" "$3" >"$out/deny" 2>&1; then echo "unexpected grant: $3"; exit 1; fi; grep -Eq 'permission denied|must be owner' "$out/deny"; }
admin <<'SQL'
CREATE ROLE gx_owner NOLOGIN;
CREATE ROLE gx_migrator LOGIN NOINHERIT;
GRANT gx_owner TO gx_migrator WITH INHERIT FALSE, SET TRUE;
CREATE ROLE gx_runtime LOGIN;
CREATE ROLE gx_token LOGIN;
CREATE ROLE gx_key LOGIN;
CREATE ROLE gx_backup LOGIN;
CREATE DATABASE gx_source OWNER gx_owner;
CREATE DATABASE gx_restore OWNER gx_owner;
GRANT CONNECT ON DATABASE gx_source,gx_restore TO gx_migrator;
SQL
canonical() { docker run --rm --network "container:$c" goformx-qualification-migration:f35a10c7 -database 'postgres://gx_migrator@127.0.0.1/gx_source?sslmode=disable&options=-c%20role%3Dgx_owner' "$@"; }
canonical goto 2026090103
probe gx_migrator gx_source "SET ROLE gx_owner; INSERT INTO forms(uuid,organization_id,title,name,public_key,status,current_schema_version) VALUES('11111111-1111-1111-1111-111111111111','22222222-2222-2222-2222-222222222222','Synthetic contact','contact','gfpk_synthetic','published',1); INSERT INTO form_schemas(uuid,form_id,schema,version,state) VALUES('33333333-3333-3333-3333-333333333333','11111111-1111-1111-1111-111111111111','{\"\u0024schema\":\"https://json-schema.org/draft/2020-12/schema\",\"type\":\"object\",\"properties\":{\"message\":{\"type\":\"string\"}}}',1,'published'); INSERT INTO form_submissions(uuid,form_id,data,schema_version,request_id) VALUES('44444444-4444-4444-4444-444444444444','11111111-1111-1111-1111-111111111111','{\"message\":\"old\"}',1,'old');" >/dev/null
canonical up
test "$(probe gx_migrator gx_source 'SET ROLE gx_owner; SELECT version::text || chr(58) || dirty::text FROM schema_migrations' | tail -n 1)" = '2026092904:false'
echo 'PASS exact migration image canonical tool populated goto/up metadata clean'
acl() { { echo 'BEGIN; SET ROLE gx_owner;'; cat goforms/deploy/postgresql/permissions.sql; echo 'COMMIT;'; } | docker exec -i "$c" psql -X -v ON_ERROR_STOP=1 -h 127.0.0.1 -U gx_migrator -d "$1" -v database="$1" -v owner=gx_owner -v migrator=gx_migrator -v runtime=gx_runtime -v token_operator=gx_token -v key_operator=gx_key -v backup=gx_backup >/dev/null; }
acl gx_source
probe gx_runtime gx_source "INSERT INTO sites(uuid,organization_id,name,origin) VALUES('55555555-5555-5555-5555-555555555555','22222222-2222-2222-2222-222222222222','Synthetic','https://synthetic.invalid'); UPDATE forms SET site_id='55555555-5555-5555-5555-555555555555'; INSERT INTO form_submissions(uuid,form_id,data,schema_version,request_id,site_id_at_acceptance) VALUES('66666666-6666-6666-6666-666666666666','11111111-1111-1111-1111-111111111111','{\"message\":\"new\"}',1,'new','55555555-5555-5555-5555-555555555555'); UPDATE forms SET site_id=NULL;" >/dev/null
if migrate goforms/migrations/postgresql/2026092904_submission_site_snapshot.down.sql gx_source >"$out/down" 2>&1; then echo 'downgrade incorrectly accepted'; exit 1; fi
grep -q 'rollback refused: accepted attribution exists' "$out/down"
test "$(probe gx_runtime gx_source 'SELECT count(*) FROM form_submissions WHERE site_id_at_acceptance IS NOT NULL')" = 1
echo 'PASS populated migration, NULL historical attribution, new snapshot retained, downgrade refused'
docker exec "$c" pg_dump -h 127.0.0.1 -U gx_backup -d gx_source -Fc --no-owner --no-privileges >"$out/source.dump"
openssl rand -base64 48 >"$out/key"
openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt -pass file:"$out/key" -in "$out/source.dump" -out "$out/backup.enc"
mkdir "$out/retrieval"
cp "$out/backup.enc" "$out/retrieval/backup.enc"
openssl enc -d -aes-256-cbc -pbkdf2 -iter 200000 -pass file:"$out/key" -in "$out/retrieval/backup.enc" -out "$out/retrieval/restored.dump"
cmp "$out/source.dump" "$out/retrieval/restored.dump"
docker exec -i "$c" pg_restore -h 127.0.0.1 -U gx_migrator --role=gx_owner -d gx_restore --no-owner --no-privileges --exit-on-error <"$out/retrieval/restored.dump"
acl gx_restore
# Forward repair path: retain upgraded schema, rebuild the acceptance index under
# maintenance ownership, then prove the same runtime can append another write.
probe gx_migrator gx_restore 'SET ROLE gx_owner; BEGIN; LOCK TABLE form_submissions IN ACCESS EXCLUSIVE MODE; DROP INDEX form_submissions_site_time_id_idx; CREATE INDEX form_submissions_site_time_id_idx ON form_submissions(site_id_at_acceptance,submitted_at DESC,uuid DESC) WHERE site_id_at_acceptance IS NOT NULL; COMMIT;' >/dev/null
probe gx_runtime gx_restore "INSERT INTO form_submissions(uuid,form_id,data,schema_version,request_id,site_id_at_acceptance) VALUES('77777777-7777-7777-7777-777777777777','11111111-1111-1111-1111-111111111111','{\"message\":\"after-forward-repair\"}',1,'after-forward-repair','55555555-5555-5555-5555-555555555555');" >/dev/null
test "$(probe gx_runtime gx_restore 'SELECT count(*) FROM form_submissions WHERE site_id_at_acceptance IS NOT NULL')" = 2
echo 'PASS fenced forward index repair preserves new write and accepts subsequent runtime write'
for db in gx_source gx_restore; do
  expected=2; [[ "$db" = gx_restore ]] && expected=3
  test "$(probe gx_runtime "$db" 'SELECT count(*) FROM form_submissions')" = "$expected"
  test "$(probe gx_runtime "$db" 'SELECT count(*) FROM form_submissions WHERE site_id_at_acceptance IS NULL')" = 1
  deny gx_runtime "$db" 'DELETE FROM form_submissions'
  deny gx_runtime "$db" 'CREATE TABLE public.forbidden(i int)'
  deny gx_runtime "$db" 'SET ROLE gx_owner'
  deny gx_backup "$db" 'UPDATE forms SET title=title'
  deny gx_backup "$db" 'CREATE TABLE public.forbidden(i int)'
  deny gx_token "$db" 'SELECT * FROM form_submissions'
  deny gx_key "$db" 'SELECT * FROM form_submissions'
  probe gx_migrator "$db" 'SET ROLE gx_owner; CREATE TABLE future_private(i int); CREATE FUNCTION future_private_fn() RETURNS int LANGUAGE sql AS $$SELECT 1$$;' >/dev/null
  deny gx_runtime "$db" 'SELECT * FROM future_private'
  deny gx_backup "$db" 'SELECT * FROM future_private'
  deny gx_runtime "$db" 'SELECT future_private_fn()'
  test "$(probe gx_migrator "$db" "SELECT count(*) FROM pg_tables WHERE schemaname='public' AND tableowner <> 'gx_owner'")" = 0
  test "$(probe gx_migrator "$db" "SELECT count(*) FROM pg_roles WHERE rolname LIKE 'gx_%' AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls)")" = 0
done
docker run --rm --network "container:$c" -e DATABASE_URL='postgres://gx_token@127.0.0.1/gx_restore?sslmode=disable' goformx-qualification-maintenance:f35a10c7 ./bin/goformx-token issue --owner 22222222-2222-2222-2222-222222222222 --scopes forms:read,submissions:read >"$out/token.json"
token=$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["token"])' "$out/token.json")
api="$c-api"
docker run -d --name "$api" --network "container:$c" -e APP_HOST=0.0.0.0 -e APP_PORT=8090 -e DB_HOST=127.0.0.1 -e DB_NAME=gx_restore -e DB_PASSWORD=disposable-trust-only -e DB_USERNAME=gx_runtime -e DB_SSL_MODE=disable -e WEBHOOK_ENABLED=false goformx-qualification-api:f35a10c7 >/dev/null
for i in $(seq 1 30); do docker exec "$api" wget -qO- http://127.0.0.1:8090/health >"$out/health" 2>/dev/null && break; sleep 1; done
docker exec "$api" wget -qO- --header="Authorization: Bearer $token" http://127.0.0.1:8090/v1/submissions >"$out/inbox.json"
python3 -c 'import json,sys; assert len(json.load(open(sys.argv[1]))["data"]) == 3' "$out/inbox.json"
port=$(docker port "$c" 8090/tcp | cut -d: -f2)
status=$(curl -sS -o "$out/submit.json" -w '%{http_code}' -H 'Content-Type: application/json' -H 'Idempotency-Key: recovery-api-post-restore-0001' --data '{"data":{"message":"after-restore"}}' "http://127.0.0.1:$port/v1/public/forms/gfpk_synthetic/submissions")
test "$status" = 202 || { cat "$out/submit.json"; exit 1; }
test "$(probe gx_runtime gx_restore 'SELECT count(*) FROM form_submissions')" = 4 || { cat "$out/submit.json"; exit 1; }
echo 'PASS exact maintenance token CLI, API restored inbox read and real public submission under runtime role'
echo 'PASS separate reader pg_dump, encrypted local retrieval, owner restore, ACL replay, role/default-denial probes'
echo 'LIMIT no external backup target, independent key custody, retention policy, binary rollback or production conversion proof'








