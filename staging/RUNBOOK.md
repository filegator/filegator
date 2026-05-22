# Staging deploy runbook

Sequential, copy-pasteable. Run as a non-root sudo user on the VM unless noted.

## 0. Before the VM exists

We're **not** copying file content from prod — UAT exercises MFA + password
reset + session handling, none of which touch file storage. Testers will
upload throwaway test files on staging.

From an SSH session on the prod box, build a small tarball containing just
`configuration.php` and the users file:

```
PROD_PATH=/var/www/filegator
sudo tar czf /tmp/filegator-snapshot.tgz \
  -C "$PROD_PATH" \
  configuration.php \
  private/users.json
sudo chown $USER:$USER /tmp/filegator-snapshot.tgz
```

Then from your laptop, pull it down and forward to the staging VM:

```
scp prod-host:/tmp/filegator-snapshot.tgz .
scp filegator-snapshot.tgz ubuntu@<STAGING_VM_IP>:/opt/filegator-staging/
ssh prod-host "sudo rm /tmp/filegator-snapshot.tgz"
```

Have your **Gmail Workspace app password** ready (the 16-char string Google showed you when you created it — strip the spaces).

> **Note on prod's storage adapter.** Prod uses a custom Flysystem adapter
> pointing at `__DIR__/filestorage`, which is itself a mountpoint for an
> external block-storage volume. We're not snapshotting that volume — staging
> starts with an empty filestorage and that's fine for UAT. The unreleased
> changes don't read or write through the storage adapter.

## 1. Provision the VM

- DigitalOcean / Hetzner / Vultr — Ubuntu 22.04 LTS, 1–2 GB RAM, smallest tier.
- Open inbound ports 22, 80, 443 in the provider's firewall.
- Note the public IPv4. Build your sslip.io hostname: `203.0.113.45` → `203-0-113-45.sslip.io`.

## 2. Install Docker + clone the repo on the VM

```
ssh ubuntu@<VM_IP>
sudo apt-get update
sudo apt-get install -y docker.io docker-compose-plugin git
sudo usermod -aG docker $USER && newgrp docker

sudo mkdir -p /opt/filegator-staging && sudo chown $USER /opt/filegator-staging
git clone https://github.com/zpaulsgrove/filegator.git /opt/filegator-staging
cd /opt/filegator-staging
git checkout master    # contains the MFA + password-reset commits
```

## 3. Unpack the production snapshot on the VM

Section 0 already SCP'd `filegator-snapshot.tgz` into `/opt/filegator-staging/`.
Unpack it into `snapshot/` and stage `configuration.php` for editing:

```
cd /opt/filegator-staging
mkdir -p snapshot
tar xzf filegator-snapshot.tgz -C snapshot
cp snapshot/configuration.php ./configuration.php
ls snapshot/      # should show: configuration.php  private/
```

## 4. Edit `configuration.php` on the VM for staging

Apply the deltas in [`configuration.staging.patch.md`](configuration.staging.patch.md):

```
cd /opt/filegator-staging
openssl rand -hex 32                              # paste into csrf_key
nano configuration.php                            # apply the 4 sections in the patch doc
```

Specifically you change:
- `csrf_key` → freshly generated value
- Mailer `dsn`, `from_email`, `from_name` → Postmark API values
- `reset_url_base` → `https://<DASHED-IP>.sslip.io/`
- `reset_subject` → prefix with `[STAGING]`
- Add the `Filegator\Services\Audit\AuditMailer` service block (operational alert emails — `staff@elliffcpa.com` recipient and From: address). Required for audit alerts to fire; leave the block out (or `enabled => false`) to silence them.

## 5. Set the sslip.io hostname for Caddy

```
echo "STAGING_HOST=<DASHED-IP>.sslip.io" > staging/.env
```

The Caddyfile reads `{$STAGING_HOST}` from the compose env.

## 6. Boot the stack

```
cd /opt/filegator-staging
docker compose -f staging/docker-compose.staging.yml --env-file staging/.env up -d --build
```

First boot: Caddy will fetch a Let's Encrypt cert (~30–60 s). Watch the logs until you see `certificate obtained successfully`:

```
docker compose -f staging/docker-compose.staging.yml logs -f caddy
```

## 7. Seed the `private/` volume with prod's users.json

The container creates an empty `private/` on first boot. Drop prod's
users.json in so testers can log in with their real accounts:

```
docker compose -f staging/docker-compose.staging.yml stop filegator

docker run --rm \
  -v filegator-staging_private:/dst \
  -v /opt/filegator-staging/snapshot/private:/src \
  alpine sh -c "cp -a /src/users.json /dst/ && chown -R 33:33 /dst"

docker compose -f staging/docker-compose.staging.yml start filegator
```

UID 33 = `www-data` inside the PHP-Apache image. The `filestorage/` volume
stays empty — testers will upload throwaway files during UAT.

## 8. Smoke-test before involving users

In a browser: `https://<DASHED-IP>.sslip.io/` → FileGator login page, valid cert, no warnings.

Pick an admin from `snapshot/users.json` whose email you control (or edit the file so one user's email is yours), then:

1. Click "Forgot password?" → enter that email
2. Watch logs: `docker compose -f staging/docker-compose.staging.yml exec filegator tail -f private/logs/app.log`
3. Email should arrive (subject `[STAGING] Reset your FileGator password`)
4. Click the link → URL host is the sslip.io address (not prod) → reset → log in
5. After login, complete the forced MFA enroll, save backup codes, log out, log back in with the authenticator

If mail fails the log will name the cause. The three near-certain culprits:
- App password copied with spaces still in it
- `@` not URL-encoded as `%40` in the DSN
- `from_email` not the same address that authenticates

## 9. Hand to business users

- Send each tester their sslip.io URL + the printed [`UAT-checklist.md`](UAT-checklist.md)
- Sit with them for the first run (script says you'll do this)
- Collect signed checklists

## 10. Decommission after sign-off

```
docker compose -f staging/docker-compose.staging.yml down -v
```

Then destroy the VM via the provider console. Staging only ever held a copy of prod's `users.json` plus throwaway test uploads — prod remains authoritative, and nothing on staging needs to be migrated back.

Promote to production by following [`docs/upgrade.md`](../docs/upgrade.md) against the existing prod domain. Same `configuration.php` merge rules, same data-preservation rules, but using prod's real domain and prod's real SMTP credentials in place of the staging values.
