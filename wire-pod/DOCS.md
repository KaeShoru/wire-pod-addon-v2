# Wire-Pod for Vector

## Optional Russian voice through local TTS

The app can request natural Russian speech from a LAN TTS service and stream it
to the robot. The TTS model does not run on the Home Assistant host.

For the companion Silero service on the Orange Pi, set these app options:

```yaml
tts_url: http://ORANGE_PI_IP:8622/tts
tts_speaker: eugene
tts_speed: 0.82
tts_profile: natural
tts_pitch: 1.0
tts_volume: 50
vector_vocoder: false
vocoder_carrier_hz: 180
vocoder_mix: 0.18
```

Leave `vector_vocoder` disabled for clear speech. The built-in vocoder is an
experimental effect rather than voice cloning and can heavily distort some
voices. When enabled, `vocoder_mix` is its main tuning control: `0` is the
natural TTS voice and `1` is the fully synthetic signal. Start around `0.18`;
higher values deliberately replace more of the clear voice. If the service is
unreachable, Wire-Pod logs the error and automatically uses Vector's native
phonetic fallback.

`tts_speed` controls the duration without changing pitch. `tts_volume` is the
external playback level sent to Vector; `50` matches the official Vector SDK
default and avoids overdriving the robot's small speaker.

For a pitch-matched source before the experimental vocoder, use
`tts_profile: vocoder_source`. This profile changes pitch and formants without
bit crushing or compression. `tts_pitch` controls the ratio: `1.10` raises
`aidar` into roughly the same fundamental range as the reference Vector voice.

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
