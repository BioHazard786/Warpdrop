# Deploying WarpDrop

So you've decided to self-host. You brave, beautiful soul. Here is how you run WarpDrop on your own iron without losing your mind (hopefully).

## The "I Just Want It To Work" Stack

By default, the `docker-compose.yml` runs:
- **Frontend**: Next.js (Port 3000, but don't touch it directly).
- **Backend**: Go Application (Port 8080).
- **Installer**: A simple service for the `curl | bash` magic (Port 8000).
- **Traefik**: The reverse proxy that handles SSL so you don't have to deal with certbot manually.

**Notable omissions**: We disabled the TURN server (`coturn`) by default because configuring it is a headache and 90% of you don't need it.

---

## Prerequisites

1.  **A Server**: A VPS, a Raspberry Pi, or an old laptop under your bed.
2.  **A Domain**: You need a real domain (e.g., `cool-file-sharing.com`) pointing to your server's IP.
3.  **Docker & Docker Compose**: If you don't have this, stop here and go learn Docker.

---

## 1. The 30-Second Setup

This assumes you are root or have sudo powers.

```bash
# 1. Clone the repo (duh)
git clone https://github.com/BioHazard786/warpdrop.git
cd warpdrop

# 2. Setup config
cp .env.example .env

# 3. Edit the config
nano .env 
# CHANGE "DOMAIN" to your actual domain.
# CHANGE "ACME_EMAIL" to your email (for Let's Encrypt).
```

### 2. Launch It 🚀

```bash
docker compose up -d --build
```

Wait about 60 seconds. Traefik needs to talk to Let's Encrypt to get your HTTPS certificates.
Visit `https://your-domain.com`. If it loads, you're a genius.

---

## 3. The "My Friends Can't Connect" Mode (Enabling TURN)

If file transfers fail between different networks (e.g., WiFi to 4G), your NAT is being difficult. You need a TURN server.

1.  **Open `docker-compose.yml`**: Scroll down to the commented-out `coturn` service.
2.  **Uncomment it**: Also uncomment the `certs-dumper` service (it's a helper to give certificates to the TURN server).
3.  **Update `.env`**: Uncomment the `NEXT_PUBLIC_TURN_SERVER` lines and set a password.
4.  **Open Ports**: You need to allow these ports on your firewall (UFW/AWS Security Group):
    - `3478` (TCP & UDP)
    - `5349` (TCP & UDP)
    - `443` (TCP - used by Traefik)
    - `80` (TCP - used by Traefik)

Then restart:
```bash
docker compose up -d ctx
```

---

## 4. The "I Hate Traefik" Option (Nginx)

If you strictly prefer Nginx, we have a sample config in `deploy/nginx.conf.sample`.
You will need to:
1.  Remove `traefik` from `docker-compose.yml`.
2.  Run your own Nginx container or host-level Nginx.
3.  Proxy `/` to port 3000.
4.  Proxy `/ws` to port 8080 (don't forget WebSocket headers).
5.  Proxy `install.yourdomain.com` to port 8000.

Good luck. You're on your own.

---

## Troubleshooting

- **"It says 404"**: Did you set the `DOMAIN` correctly in `.env`?
- **"SSL Error"**: Check `docker compose logs traefik`. If Let's Encrypt can't reach port 80, no certs for you.
- **"Transfer stuck at 0%"**: You definitely need the TURN server. See step 3.
