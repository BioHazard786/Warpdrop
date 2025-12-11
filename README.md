# WarpDrop

Self-deployable, privacy-friendly, realtime file sharing over WebRTC with a tiny Go signaling server and a Next.js (Bun) frontend.

## Highlights

- Self-deploy in minutes on any VPS using Docker Compose
- HTTPS out of the box with Traefik (or optional Nginx)
- WebSocket signaling (`/ws`), peer-to-peer data channels for files
- No central storage: files stream directly, ephemeral by design
- Lightweight images: distroless Go backend, Bun-powered frontend

## Quick Start (Local)

```bash
# from project root
cp .env.example .env
# build images
docker compose build
# start services
docker compose up -d
# tail logs
docker compose logs -f traefik
```

- Frontend: `http://localhost` (through Traefik) or attach Traefik labels later.
- Backend health: `curl http://localhost:8080/` returns "Signaling server is healthy."

## Production Deployment (VPS)

1. Point your domain DNS (A/AAAA) to the VPS IP.
2. Install Docker + Compose v2.
3. Copy `.env` and set values:

```env
DOMAIN=yourdomain.com
ACME_EMAIL=your-email@example.com

NEXT_PUBLIC_STUN_SERVER=stun:stun.l.google.com:19302

NEXT_PUBLIC_TURN_SERVER=yourdomain.com
NEXT_PUBLIC_TURN_USERNAME=warpdrop
NEXT_PUBLIC_TURN_PASSWORD=warpdrop-secret
```

4. Bring up the stack:

```bash
docker compose up -d --build
```

5. Verify certificates and routes:

```bash
docker compose logs -f traefik
```

6. Open firewall ports on your VPS:

| Port | Protocol | Service | Purpose |
|------|----------|---------|---------|
| 80 | TCP | Traefik | HTTP (Let's Encrypt ACME challenge) |
| 443 | TCP | Traefik | HTTPS (frontend + backend) |
| 3478 | UDP | Coturn | TURN relay (media) |
| 3478 | TCP | Coturn | TURN relay (fallback) |
| 5349 | UDP | Coturn | TURN TLS (media) |
| 5349 | TCP | Coturn | TURN TLS (fallback) |

UFW example (Linux):

```bash
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 3478/udp
sudo ufw allow 3478/tcp
sudo ufw allow 5349/udp
sudo ufw allow 5349/tcp
```

Or use your cloud provider's security group (AWS, DigitalOcean, Linode, etc.).

- Routing model:
  - `https://yourdomain.com` → frontend (Next.js on port 3000)
  - `https://yourdomain.com/ws` → backend (Go on port 8080, WebSocket)

### Nginx Option

Prefer Nginx TLS termination? Use `deploy/nginx.conf.sample` and route:

- `/` → `frontend:3000`
- `/ws` → `backend:8080/ws` (WebSocket headers included)

If you pick Nginx, you can remove Traefik from `docker-compose.yml`.

## Configuration

### Environment Variables

Copy `.env.example` to `.env` and configure:

```env
# Domain for your deployment
DOMAIN=yourdomain.com
ACME_EMAIL=your-email@example.com

# STUN server (helps clients discover their public IP)
NEXT_PUBLIC_STUN_SERVER=stun:stun.l.google.com:19302

# TURN server (relays traffic for clients behind restrictive NATs)
NEXT_PUBLIC_TURN_SERVER=yourdomain.com
NEXT_PUBLIC_TURN_USERNAME=warpdrop
NEXT_PUBLIC_TURN_PASSWORD=warpdrop-secret
```

### ICE Server Configuration

WarpDrop includes:

- **STUN**: Helps peers discover their public IP and establish direct connections
- **TURN**: Relays traffic for peers that can't connect directly (restrictive firewalls/NATs)

The TURN server (coturn) is automatically included in the Docker Compose stack and uses your configured domain. For best connectivity, keep both enabled.

### Services

- Frontend scripts (Bun): `bun run build`, `bun run start` (port 3000)
- Backend (Go): serves `/` (health) and `/ws` (WebSocket signaling) on 8080
- Traefik: ACME HTTP challenge, automatic HTTP→HTTPS redirect, security headers middleware via `deploy/traefik_dynamic.yml`
- Coturn: TURN/STUN relay server on ports 3478 (TCP/UDP) and 5349 (TLS)

## Development

```bash
# Frontend (dev)
cd frontend
bun install
bun run dev

# Backend (dev)
cd backend
go run ./cmd/server
```

## Project Structure

- `frontend/` Next.js app (Bun)
- `backend/` Go signaling server
- `deploy/` Traefik dynamic config, Nginx sample, Turn Server config
- `docker-compose.yml` Services: frontend, backend, traefik

## Security Notes

- HTTPS enabled via Let's Encrypt (Traefik) or via Nginx with your certs
- WebRTC peer-to-peer transfers; server never stores file content
- HSTS, XSS filter, frame deny headers applied by Traefik middleware

## Roadmap / TODOs

- [x] CLI tool for quick sharing via terminal
  - ✅ implemented
- [ ] Support for Multiple Receivers
  - ❌ not yet implemented
- [ ] Mobile App (Optional)
  - ❌ not yet implemented

## Troubleshooting

- Certs fail to issue: ensure port 80 is reachable (ACME HTTP challenge) and DNS points to your VPS.
- WebSocket `101` upgrade missing: confirm proxy passes `Upgrade` and `Connection` headers (Traefik handles automatically; Nginx sample provided).
- Frontend 404s: check domain in `.env` and Traefik labels.
- TURN server connection fails: ensure ports 3478 (UDP/TCP) and 5349 (TLS) are open in your firewall, and `NEXT_PUBLIC_TURN_SERVER` matches your domain.
- WebRTC connection issues: check browser console for ICE candidate errors. TURN server fallback ensures connectivity even with restrictive NATs.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Related Efforts

- [`FilePizza`](https://github.com/kern/filepizza) - Inspiration for peer-to-peer file sharing over WebRTC.
- [`fs-cli`](https://github.com/spectre10/fs-cli) - Inspiration for a Go-based CLI for peer-to-peer file sharing over WebRTC.

## Acknowledgments

- [`Traefik`](https://traefik.io/) - for effortless reverse proxy and TLS management.
- [`Bun`](https://bun.sh/) - for blazing fast JavaScript runtime for the frontend.
- [`Go`](https://golang.org/) - for a powerful and efficient backend server.
- [`Next.js`](https://nextjs.org/) - for the React framework powering the frontend.
- [`Shadcn`](https://ui.shadcn.com/) - for UI components and design inspiration.
- [`Origin UI`](https://coss.com/origin) - for sleek file upload and table components.
