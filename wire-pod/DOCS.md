# Wire-Pod for Vector

Wire-Pod runs the local voice services used by Anki Vector. This Home Assistant
app uses the official Wire-Pod container and keeps its state in Home Assistant's
persistent `/data` volume.

## Before starting

The app uses host networking. The following ports must be free on the Home
Assistant host:

| Port | Purpose |
| --- | --- |
| `80/tcp` | Vector connectivity check |
| `443/tcp` | Vector cloud replacement services |
| `8080/tcp` | Wire-Pod setup and configuration UI |
| `8084/tcp` | Compatibility endpoint for newer Vector firmware |

If another app or reverse proxy already listens on port 80 or 443, stop it or
move it before starting Wire-Pod.

## First start

1. Start the app.
2. Wait until the log says that Wire-Pod is not set up and the web server is
   available on port 8080.
3. Select **Open Web UI** or **Show in sidebar** to use the authenticated Home
   Assistant Ingress panel. Direct access remains available at
   `http://<home-assistant-host>:8080` for Vector setup tools that need it.
4. Complete Wire-Pod's setup wizard.
5. Make sure Vector and Home Assistant are on the same LAN and that client/AP
   isolation is disabled.

The app is healthy when `http://<home-assistant-host>:8080/ok` responds.

## Memory-conscious settings

- Start with Vosk and the smallest model for the language you use.
- Avoid local Whisper on low-memory hardware. It has a significantly larger
  memory and CPU footprint.
- Configure and test one robot before adding integrations or custom intents.
- Keep at least several hundred megabytes of free storage for the image, model
  and persistent state.

The Ingress proxy is intentionally small. The optional Vector bridge runs as a
separate app, so camera/control features can be stopped without interrupting
Vector's voice server.

## Vector sensors and commands

Install **Vector (Anki / Digital Dream Labs)** from this repository and leave
its `wirepod_url` option set to `auto`. Then install the
[Vector integration](https://github.com/witold-gren/home-assistant-vector-integration)
through HACS. Home Assistant will discover the bridge automatically.

This provides battery percentage and voltage, charger and sleep state, robot
events, camera and speaker entities, and behavior buttons including go to
charger and go to sleep. It also exposes movement, animations, photos, faces,
cube actions and optional Voice → Home Assistant routing. MQTT is not needed.

## Persistence and backups

Wire-Pod certificates, robot configuration, speech assets and custom intents
are stored under `/data`. Home Assistant uses a cold backup for this app: it
briefly stops Wire-Pod so the backup is internally consistent.

Restoring the app backup restores the Wire-Pod state. Do not expose backup files
because they can contain robot certificates and API credentials.

## Troubleshooting

### The app cannot start

Check the app log first. The most common cause is another process using port 80,
443, 8080 or 8084.

### The UI opens but Vector cannot connect

- Confirm Vector and Home Assistant are on the same subnet.
- Disable Wi-Fi client isolation or guest-network isolation.
- Verify that local multicast/mDNS traffic is allowed.
- Check that the Wire-Pod setup wizard completed and generated certificates.

### Installation fails before the app starts

Release installation should pull a pre-built image. If Home Assistant attempts
to compile Wire-Pod, the release image is missing or inaccessible. Verify that
the `ghcr.io/kaeshoru/wire-pod-addon-v2` package is public.

### Updating the upstream Wire-Pod version

The upstream image is deliberately pinned by digest. Updating it requires a
new app version and a tested digest change; restarting or reinstalling the same
version will not silently replace Wire-Pod with newer upstream code.
