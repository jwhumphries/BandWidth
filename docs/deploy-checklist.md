# Deploy checklist — first push to fly.io

The deploy pipeline is already built (`fly.toml`, `.github/workflows/publish.yml`,
the Dagger `Publish` function) and the GHCR image is public. What's left is the
one-time external setup.

Public hostname: **https://bandwidth.jwhlabs.dev** (set in `fly.toml [env]` as
`BANDWIDTH_BASE_URL`).

## 1. Log in to Fly

```bash
flyctl auth login
```

## 2. Create the app and volume

```bash
fly apps create bandwidth
fly volumes create bandwidth_data --region ord --size 1
```

`bandwidth` and `bandwidth_data` match `app` and the `[[mounts]]` source in
`fly.toml`. Keep it to one machine — SQLite is single-writer.

## 3. (Optional) SMTP secrets

Email (invites / password reset) is skipped if these are unset:

```bash
fly secrets set BANDWIDTH_SMTP_HOST=... BANDWIDTH_SMTP_PORT=587 \
  BANDWIDTH_SMTP_USER=... BANDWIDTH_SMTP_PASS=... BANDWIDTH_SMTP_FROM=...
```

`BANDWIDTH_BASE_URL` is already in `fly.toml`, so no base-URL secret is needed.

## 4. Add the deploy token as a GitHub secret

The `deploy-to-fly` job needs `FLY_API_TOKEN` (currently unset):

```bash
fly tokens create deploy
gh secret set FLY_API_TOKEN -R jwhumphries/BandWidth   # paste the token
```

## 5. Custom domain — bandwidth.jwhlabs.dev (Cloudflare, A/AAAA)

Fly issues a Let's Encrypt cert; Cloudflare points A/AAAA records at the app's
Fly IPs (mirrors the blog setup). The IPs aren't assigned until the app exists,
so do this after step 2 (and ideally after the first deploy in step 6).

1. Get the app's Fly IPs:
   ```bash
   fly ips list -a bandwidth
   ```
   This shows a dedicated IPv6 and a shared IPv4 (both free). A shared IPv4 is
   fine — Fly routes it by SNI/Host. (Allocate explicitly if none are listed:
   `fly ips allocate-v6 -a bandwidth` and `fly ips allocate-v4 --shared -a bandwidth`.)

2. Tell Fly about the hostname:
   ```bash
   fly certs add bandwidth.jwhlabs.dev -a bandwidth
   fly certs show bandwidth.jwhlabs.dev -a bandwidth   # check issuance status
   ```

3. In the Cloudflare dashboard for **jwhlabs.dev**, add two DNS records:
   - **A** — Name `bandwidth` → the IPv4 from step 1
   - **AAAA** — Name `bandwidth` → the IPv6 from step 1
   - **Proxy status:** **DNS only (grey cloud)** at first — Cloudflare's orange-cloud
     proxy intercepts the ACME challenge and can block cert issuance.

4. Wait for the cert. Re-run `fly certs show bandwidth.jwhlabs.dev -a bandwidth`
   until status is **Issued / Ready**.

5. (Optional) Once the cert is issued you may flip the records to
   **Proxied (orange cloud)** for Cloudflare's CDN/WAF. If you do, set Cloudflare
   SSL/TLS mode to **Full (strict)** so it validates Fly's cert.

## 6. Deploy

Deploys fire on push to `main` only. Current work lives on `dev`, so:

```bash
git checkout main && git merge dev && git push
```

That runs `publish.yml`: Dagger builds + pushes the image to GHCR, then
`flyctl deploy --remote-only` rolls it out. Manual alternative:
`fly deploy --remote-only`.

After the first deploy, verify:

```bash
fly status -a bandwidth
curl -sS https://bandwidth.jwhlabs.dev/healthz
```
