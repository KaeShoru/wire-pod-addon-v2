# Changelog

## 2.5.0

- Reworked the experimental vocoder to follow the source speech melody: a
  30 ms autocorrelation pitch tracker (the approach documented for Vector's
  Krotos Dehumaniser) drives the harmonic carrier, instead of a fixed-tone
  carrier that flattened all intonation into a drone. Unvoiced sounds
  (fricatives, pauses) now use a noise carrier.
- `vocoder_carrier_hz` is now only the fallback frequency for fully unvoiced
  input.

## 2.4.1

- Added the RHVoice engine choice (`tts_engine: rhvoice`): statistical
  parametric Russian voices — clearly synthetic but smooth and intelligible,
  the classic screen-reader robot sound. `tts_variant` picks the RHVoice
  voice (e.g. `aleksandr`, `anna`, `elena`, `irina`; empty = `aleksandr`).

## 2.4.0

- Added a selectable TTS engine: `silero` (neural voices) or `espeak`
  (classic formant synthesis with robotic variants such as `klatt`, `m3`,
  `croak`, `whisper`) via the new `tts_engine` and `tts_variant` options.
- Raised the TTS volume ceiling from 100 to 200: values above 100 amplify the
  PCM with a soft limiter instead of hard clipping, so quiet voices can be
  boosted without crackling.

## 2.3.5

- Added configurable TTS profile and pitch parameters for preparing the source
  voice before the experimental vocoder.
- Added support for the companion Silero `vocoder_source` profile, which shifts
  pitch and formants without adding compression or bit crushing.

## 2.3.4

- Fixed the experimental vocoder wet/dry path so its saturation applies only
  to the synthetic layer instead of compressing the original TTS voice.
- Reduced the suggested vocoder mix from 0.82 to 0.18 and made a zero mix an
  exact, inexpensive pass-through.

## 2.3.3

- Added a configurable external TTS playback volume and changed its default
  from the previously hard-coded maximum of 100 to the Vector SDK default of
  50, reducing harsh compression and speaker distortion.
- Reduced the default Silero speed from 0.96 to 0.82 for clearer Russian speech.

## 2.3.2

- Disabled the experimental vocoder by default so local Russian TTS is clear
  and natural on first use.
- Made the vocoder opt-in: an absent or empty setting can no longer enable the
  effect accidentally.

## 2.3.1

- Accept streaming WAV responses whose `data` length is the FFmpeg
  `0xffffffff` sentinel instead of rejecting them and falling back to native
  phonetic speech.

## 2.3.0

- Added optional local Russian TTS through a configurable HTTP endpoint.
- Added a lightweight 20-band vocoder based on Vector's original
  `Krotos_Vocoder_12` soundbank settings: 180 Hz carrier, 5 ms envelopes and a
  -2 dB output ceiling.
- Kept the built-in phonetic voice as an automatic fallback when the external
  TTS service is disabled or unavailable.
- The Home Assistant host only performs inexpensive DSP; the TTS model remains
  on a separate machine such as an Orange Pi.

## 2.2.0

- Added automatic Russian-to-Latin phonetic conversion before native Vector speech.
- Kept Vector's built-in voice; no external TTS or Home Assistant relay is required.
- Made OpenAI-compatible streaming tolerant of usage-only chunks and replies without final punctuation.

## 2.1.0

- Added Home Assistant Ingress with a Wire-Pod entry for the sidebar.
- Added a small authenticated reverse proxy that adapts Wire-Pod's absolute
  web paths to the Home Assistant Ingress path.
- Added the companion Vector bridge app to this repository for battery and
  state sensors, events, camera, speaker and robot commands.

## 2.0.2

- Made the Wire-Pod server the container's managed foreground process so Home
  Assistant can track and stop it reliably.
- Enabled Docker's lightweight init process for signal forwarding and zombie
  process cleanup.

## 2.0.1

- Fixed the Web UI URL template so Home Assistant Supervisor accepts and lists
  the app in the store.
- Promoted the app from experimental to the default stable stage.

## 2.0.0

- Rebuilt the repository using the supported Home Assistant app layout.
- Replaced the non-starting Python/bashio container with the official Wire-Pod
  multi-architecture image.
- Pinned the upstream image by digest for reproducible releases.
- Removed local Wire-Pod, Vosk and Go compilation from installation.
- Persisted all upstream Wire-Pod state in `/data`.
- Added host networking, health monitoring and cold-backup metadata.
- Limited supported architectures to the upstream-tested `aarch64` and `amd64`.
