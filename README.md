# WarpDrop

Privacy-first, self-hostable, realtime file sharing. Like AirDrop, but for people who don't trust "The Cloud™️" or just want to feel like a hacker sending files over a CLI.

- **Web**: [warpdrop.qzz.io](https://warpdrop.qzz.io)
- **CLI**: `curl -fsSL install.warpdrop.qzz.io | bash`
  - **Scoop (Windows)**: `scoop bucket add biohazard786 https://github.com/BioHazard786/scoop-bucket.git && scoop install biohazard786/warpdrop`
  - **Brew (MacOS)**: `brew tap BioHazard786/tap && brew install --cask warpdrop`
- **Self-Hosting**: [DEPLOY.md](DEPLOY.md) (Because you're an adult and you can host your own servers).

---

## Why WarpDrop?

Because sending files shouldn't involve uploading them to a billionaire's server farm. 

- **Peer-to-Peer**: The file goes from your machine to theirs. No stops in between.
- **Ephemeral**: It's like SnapChat for zip files. Close the tab, and it's gone forever.
- **Go + Next.js**: Backend is a tiny Go binary (because we like speed), Frontend is Next.js (because we like suffering... just kidding, it's actually pretty nice).
- **STUN/TURN**: We baked these in so you can punch through NATs like they're wet paper bags.

### ⚠️ A Note on the Demo Server

The public instance ([warpdrop.qzz.io](https://warpdrop.qzz.io)) runs on an **Oracle Cloud Free Tier A1 Instance**. 
Why? because I'm cheap. 
If it's down, Oracle probably reclaimed my instance to mine crypto or something. 
**Solution**: Host it yourself (see below).

---

## The "Look Ma, No Hands" Interoperability

This is the cool part. You can send from your terminal and receive on your phone's browser. Or vice versa. It's magic (it's actually WebRTC Data Channels, but let's call it magic).

### Scenario 1: The CLI Hacker
You're in a terminal. You haven't seen a GUI in weeks.

```bash
# Install the magic
curl -fsSL install.warpdrop.qzz.io | bash

# Send your top-secret manifesto
warpdrop send ./manifesto.txt
# Output: "Room ID: lantern-poppy-brave-peter"
```

Then yell "lantern-poppy-brave-peter" across the office. Your colleague opens [warpdrop.qzz.io](https://warpdrop.qzz.io), types it in, and BOOM. File received.

### Scenario 2: The Receiver
You run `warpdrop receive lantern-poppy-brave-peter` and the file flies into your current directory. 

---

## Self-Hosting (Be Your Own Cloud)

Don't trust my Oracle instance? Good. I wouldn't either.

1.  **Copy the env**: `cp .env.example .env`
2.  **Fill it out**: `DOMAIN=yourstuff.com`, etc.
3.  **Launch**: `docker compose up -d --build`

See [DEPLOY.md](DEPLOY.md) for the "I need to configure Nginx manually because I enjoy pain" guide.

> **Pro Tip**: You really need the TURN server (included in the docker-compose) if you want this to work between system behind symmetric NATs. Without it, you're at the mercy of the NAT gods.

---

## TODO (The "Eventually" List)

- [ ] **Multiple Receivers**: Add support for multiple receivers.
- [ ] **Mobile App**: Add a mobile app using Expo.
- [ ] **Better Logo**: If somebody is willing to donate a logo, I'll use it.
---

## Credits

Inspired by [FilePizza](https://file.pizza) (yum), [fs-cli](https://github.com/spectre10/fs-cli), and [croc](https://github.com/schollz/croc).
Reimagined because I wanted to learn WebRTC and I needed a project to procrastinate on my actual work.
