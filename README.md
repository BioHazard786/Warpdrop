# WarpDrop

Privacy-first, self-hostable, realtime file sharing over WebRTC. Use the hosted web app or your own infra, and keep transfers ephemeral by design.

- Web app: [warpdrop.qzz.io](https://warpdrop.qzz.io)
- CLI: `curl install.warpdrop.qzz.io | bash`
- Self-hosting guide: [DEPLOY.md](DEPLOY.md)

## Why WarpDrop

- Peer-to-peer streams; no file storage on servers
- Tiny Go signaling server and Bun/Next.js frontend
- STUN and TURN baked in for reliable connectivity
- Works great on a VPS, on localhost, or at the hosted URL

## Get the app

### Web

- Visit [warpdrop.qzz.io](https://warpdrop.qzz.io) to send or receive without installing anything.

### CLI

```bash
curl install.warpdrop.qzz.io | bash
# send a file
warpdrop send ./file.zip
# receive (room id shown by sender)
warpdrop receive <room-id>
```

## Self-hosting

- Copy .env.example to .env and fill domain + TURN/STUN values
- Run `docker compose up -d --build`
- See [DEPLOY.md](DEPLOY.md) for VPS and local walkthroughs (Traefik, Nginx option, firewall ports).

## Trust and safety

- Ephemeral by design: streams over WebRTC data channels, no central storage
- TLS termination via Traefik or Nginx; HSTS and sane security headers
- TURN fallback keeps transfers working behind strict NATs

## Credits

- Inspired by [FilePizza](https://file.pizza), [fs-cli](https://github.com/spectre10/fs-cli), and [croc](https://github.com/schollz/croc), reimagined for quick self-hosting and a polished web + CLI experience.
