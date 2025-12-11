# Deploy WarpDrop

Opinionated notes for running WarpDrop on a VPS or locally with Docker Compose. The stack ships Traefik + coturn by default, with an optional Nginx handoff.

## What you get

- Frontend: Next.js (Bun) on port 3000
- Backend: Go signaling on port 8080 (`/` health, `/ws` WebSocket)
- Traefik: HTTPS + security headers + ACME
- Coturn: TURN/STUN on 3478 (TCP/UDP) and 5349 (TLS)

## Prerequisites

- Docker + Docker Compose v2
- VPS only: a domain pointing to the server (A/AAAA). ACME needs port 80 reachable.
- Open these ports: 80, 443, 3478 (TCP/UDP), 5349 (TCP/UDP). See firewall snippet below.

## 1) Configure environment

Copy the template and set the basics:

```bash
cp .env.example .env
```

Key values:

```env
DOMAIN=yourdomain.com
ACME_EMAIL=you@example.com
NEXT_PUBLIC_STUN_SERVER=stun:stun.l.google.com:19302
NEXT_PUBLIC_TURN_SERVER=your-turn-server.com
NEXT_PUBLIC_TURN_USERNAME=warpdrop
NEXT_PUBLIC_TURN_PASSWORD=warpdrop-secret
```

## 2) Deploy to a VPS (with Traefik)

```bash
# from project root
cp .env.example .env  # if not done yet
# edit .env with your domain + email

docker compose up -d --build

docker compose logs -f traefik  # wait for ACME cert OK
```

- Routes: `https://yourdomain.com` → frontend, `https://yourdomain.com/ws` → backend.
- Health check: `curl https://yourdomain.com/health` return the health message on.
- Firewall example (UFW):

```bash
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 3478/udp
sudo ufw allow 3478/tcp
sudo ufw allow 5349/udp
sudo ufw allow 5349/tcp
```

### Nginx option

If you prefer Nginx TLS termination, use [deploy/nginx.conf.sample](deploy/nginx.conf.sample), route `/` to `frontend:3000` and `/ws` to `backend:8080/ws`, and remove Traefik from `docker-compose.yml`.

## 3) Run locally (no domain required)

Local HTTPS is optional; simplest path is HTTP:

```bash
cp .env.example .env
# set DOMAIN=localhost and keep the default STUN/TURN entries

docker compose up -d --build
```

- Frontend: [http://localhost](http://localhost)
- Backend health: [http://localhost:8080/](http://localhost:8080/)
- If you want local HTTPS, point a DNS override (e.g., /etc/hosts) to a custom domain and let Traefik issue certs via that hostname.

## 4) Upgrades and maintenance

- Update images: `git pull` then `docker compose pull && docker compose up -d`
- View logs: `docker compose logs -f --tail=100`
- Reset (destructive): `docker compose down -v`

## 5) Quick troubleshooting

- ACME fails: confirm port 80 reachability and correct DOMAIN. Check `docker compose logs traefik`.
- WebSocket 101 missing: ensure your proxy (Traefik/Nginx) passes `Upgrade`/`Connection` headers; use the sample configs.
- TURN unreachable: double-check 3478/5349 TCP+UDP in your firewall/security group.
- Slow peers: TURN may be relaying; that is expected behind strict NATs.
