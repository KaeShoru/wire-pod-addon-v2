package wirepod_ttr

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sync"
)

const vectorAudioRate = 16000

// vocoderRate is the firmware audio engine's internal sample rate: Vector's
// Acapela voices are 22 kHz, and the vocoder's EQ bands only land on their
// true frequencies at this rate.
const vocoderRate = 22050

// decodePCM16WAV reads the ordinary PCM WAV returned by local TTS services.
// Mono and stereo are accepted; stereo is downmixed before processing.
func decodePCM16WAV(data []byte) ([]int16, int, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, errors.New("TTS response is not a RIFF/WAVE file")
	}
	var format, channels, bits uint16
	var rate uint32
	var pcm []byte
	for offset := 12; offset+8 <= len(data); {
		name := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		if size < 0 || size > len(data)-offset {
			// WAV written to a non-seekable pipe cannot have its final data
			// length patched into the header. FFmpeg represents that as
			// 0xffffffff; in that case the data block is the remaining input.
			if name != "data" {
				return nil, 0, errors.New("invalid WAV chunk length")
			}
			size = len(data) - offset
		}
		switch name {
		case "fmt ":
			if size < 16 {
				return nil, 0, errors.New("invalid WAV fmt chunk")
			}
			format = binary.LittleEndian.Uint16(data[offset : offset+2])
			channels = binary.LittleEndian.Uint16(data[offset+2 : offset+4])
			rate = binary.LittleEndian.Uint32(data[offset+4 : offset+8])
			bits = binary.LittleEndian.Uint16(data[offset+14 : offset+16])
		case "data":
			pcm = data[offset : offset+size]
		}
		offset += size + size%2
	}
	if format != 1 || (channels != 1 && channels != 2) || bits != 16 || rate < 8000 || len(pcm) == 0 {
		return nil, 0, fmt.Errorf("unsupported WAV: format=%d channels=%d bits=%d rate=%d", format, channels, bits, rate)
	}
	frameBytes := int(channels) * 2
	frames := len(pcm) / frameBytes
	out := make([]int16, frames)
	for i := range out {
		left := int32(int16(binary.LittleEndian.Uint16(pcm[i*frameBytes:])))
		if channels == 2 {
			right := int32(int16(binary.LittleEndian.Uint16(pcm[i*frameBytes+2:])))
			left = (left + right) / 2
		}
		out[i] = int16(left)
	}
	return out, int(rate), nil
}

func resamplePCM16(input []int16, from, to int) []int16 {
	if len(input) == 0 || from <= 0 || to <= 0 || from == to {
		return append([]int16(nil), input...)
	}
	count := int(math.Round(float64(len(input)) * float64(to) / float64(from)))
	if count < 1 {
		return nil
	}
	out := make([]int16, count)
	ratio := float64(from) / float64(to)
	for i := range out {
		position := float64(i) * ratio
		lo := int(position)
		if lo >= len(input)-1 {
			out[i] = input[len(input)-1]
			continue
		}
		fraction := position - float64(lo)
		value := float64(input[lo])*(1-fraction) + float64(input[lo+1])*fraction
		out[i] = int16(math.Round(value))
	}
	return out
}

func pcm16Bytes(samples []int16) []byte {
	out := make([]byte, len(samples)*2)
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(out[i*2:], uint16(sample))
	}
	return out
}

// ---------------------------------------------------------------------------
// Vector voice replica: Krotos Vocoder as configured in Vector firmware
// 1.6.0.3331 (Victor_VO.bnk, shareset 2051987598), reverse engineered from
// libaudio_engine.so.
//
//   - 16 kHz, FFT frame 1024, hop 512, sqrt-Hann analysis+synthesis (COLA)
//   - carrier: the exact 4096-entry clipped-sinc wavetable (carrier waveform
//     enum 14), frequency = tracked modulator pitch clamped to [100, 200] Hz
//     with a 512-sample glide on every change
//   - modulator spectral envelope: per-bin |FFT| magnitude (the original's
//     5 ms followers are instant at the 31.25 Hz frame rate)
//   - carrier graphic EQ: 8 log-spaced bands over 1024 bins, gains
//     [0, 0, 0, 0, 0.5, 1, 0.5, 1], edges bin(b) = exp(b*ln(512)/7)-1
//   - mix: dominant wet layer over a quiet dry underlay
//
// On top of the firmware behaviour the replica adds a mild spectral valley
// expansion and a soft silence gate, because the local Russian TTS voices are
// breathier than the original Acapela modulator; without them the carrier
// harmonics amplify the TTS noise floor into audible hash.
// ---------------------------------------------------------------------------

const (
	vocoderFrame    = 1024
	vocoderHop      = 512
	vocoderGlide    = 512
	vocoderPitchMin = 100.0
	// The firmware clamps its internal tracker at 200 Hz, but Anki drove the
	// carrier RTPC from the outside and real Vector speech peaks near 220 Hz,
	// so our own tracker-driven contour gets more headroom.
	vocoderPitchMax = 260.0
)

var vocoderEQBandGains = [8]float64{0, 0, 0, 0, 0.5, 1, 0.5, 1}

// vocoderEQBandConst is the measured constant behind the band edge formula
// (approximates ln(512)).
const vocoderEQBandConst = 6.234410762786865

// fftTables holds precomputed bit-reversal indices and twiddle factors for
// the 1024-point transforms.
var fftTables = struct {
	sync.Once
	rev     [vocoderFrame]int
	twiddle [vocoderFrame / 2]complex128
}{}

func initFFT() {
	fftTables.Do(func() {
		bits := 10 // log2(1024)
		for i := 0; i < vocoderFrame; i++ {
			r := 0
			for b := 0; b < bits; b++ {
				r |= (i >> b & 1) << (bits - 1 - b)
			}
			fftTables.rev[i] = r
		}
		for k := 0; k < vocoderFrame/2; k++ {
			angle := -2 * math.Pi * float64(k) / vocoderFrame
			fftTables.twiddle[k] = complex(math.Cos(angle), math.Sin(angle))
		}
	})
}

// fft1024 performs an in-place iterative radix-2 FFT. When inverse is true
// the result is the unnormalised inverse transform; callers scale by 1/N.
func fft1024(buf []complex128, inverse bool) {
	initFFT()
	for i := 0; i < vocoderFrame; i++ {
		j := fftTables.rev[i]
		if j > i {
			buf[i], buf[j] = buf[j], buf[i]
		}
	}
	for size := 2; size <= vocoderFrame; size *= 2 {
		half := size / 2
		step := vocoderFrame / size
		for start := 0; start < vocoderFrame; start += size {
			for k := 0; k < half; k++ {
				w := fftTables.twiddle[k*step]
				if inverse {
					w = complex(real(w), -imag(w))
				}
				even := buf[start+k]
				odd := buf[start+k+half] * w
				buf[start+k] = even + odd
				buf[start+k+half] = even - odd
			}
		}
	}
}

var sqrtHannWindow [vocoderFrame]float64
var sqrtHannOnce sync.Once

func vocoderWindow() *[vocoderFrame]float64 {
	sqrtHannOnce.Do(func() {
		for i := 0; i < vocoderFrame; i++ {
			// np.hanning(N+2)[1:-1] equivalent, then square root.
			h := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i+1)/float64(vocoderFrame+1))
			sqrtHannWindow[i] = math.Sqrt(h)
		}
	})
	return &sqrtHannWindow
}

var vocoderEQ [vocoderFrame]float64
var vocoderEQOnce sync.Once

func carrierEQGains() *[vocoderFrame]float64 {
	vocoderEQOnce.Do(func() {
		edges := [9]int{}
		for b := 0; b <= 8; b++ {
			edges[b] = int(math.Round(math.Exp(float64(b)*vocoderEQBandConst/7) - 1))
		}
		for b := 0; b < 8; b++ {
			lo := edges[b]
			if lo < 1 {
				lo = 1
			}
			hi := 512
			if b < 7 {
				hi = edges[b+1]
			}
			if hi > 512 {
				hi = 512
			}
			for k := lo; k < hi; k++ {
				vocoderEQ[k] = vocoderEQBandGains[b]
				mirror := vocoderFrame - 1 - k
				if mirror >= 0 && mirror < vocoderFrame {
					vocoderEQ[mirror] = vocoderEQBandGains[b]
				}
			}
		}
	})
	return &vocoderEQ
}

// trackPitch estimates the fundamental frequency of 16 kHz speech once per
// hop using Hann-windowed normalised autocorrelation with parabolic
// interpolation, then median-3 smoothing. It returns one value per hop; 0
// marks unvoiced or silent frames.
func trackPitch(samples []int16, rate int) []float64 {
	frame := vocoderFrame
	hop := vocoderHop
	minLag := rate / 400
	maxLag := rate / 70
	if len(samples) < frame || maxLag >= frame {
		return nil
	}
	// Hann window lowers the autocorrelation sidelobes that otherwise pick
	// octave-wrong lags.
	win := make([]float64, frame)
	for i := range win {
		win[i] = 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(frame-1))
	}
	silenceRMS := 0.005 * 32768 // float threshold, in int16 units
	var contour []float64
	for start := 0; start+frame <= len(samples); start += hop {
		var energy float64
		for i := 0; i < frame; i++ {
			v := float64(samples[start+i])
			energy += v * v
		}
		if math.Sqrt(energy/float64(frame)) < silenceRMS {
			contour = append(contour, 0)
			continue
		}
		// Raw autocorrelation of the windowed frame, normalized by ac(0) —
		// same biased estimator the reference Python replica uses.
		ac0 := 0.0
		for i := 0; i < frame; i++ {
			v := float64(samples[start+i]) * win[i]
			ac0 += v * v
		}
		if ac0 <= 0 {
			contour = append(contour, 0)
			continue
		}
		ac := func(lag int) float64 {
			var sum float64
			for i := 0; i+lag < frame; i++ {
				sum += float64(samples[start+i]) * win[i] * float64(samples[start+i+lag]) * win[i+lag]
			}
			return sum / ac0
		}
		bestLag, bestValue := 0, 0.3
		for lag := minLag; lag <= maxLag; lag++ {
			if value := ac(lag); value > bestValue {
				bestValue, bestLag = value, lag
			}
		}
		if bestLag == 0 {
			contour = append(contour, 0)
			continue
		}
		// Parabolic interpolation around the autocorrelation peak.
		refined := float64(bestLag)
		if bestLag > minLag && bestLag < maxLag {
			a, b, c := ac(bestLag-1), ac(bestLag), ac(bestLag+1)
			if denom := a - 2*b + c; math.Abs(denom) > 1e-12 {
				refined = float64(bestLag) + 0.5*(a-c)/denom
			}
		}
		contour = append(contour, float64(rate)/refined)
	}
	for i := 1; i < len(contour)-1; i++ {
		a, b, c := contour[i-1], contour[i], contour[i+1]
		if a > b {
			a, b = b, a
		}
		if b > c {
			b, c = c, b
		}
		if a > b {
			a, b = b, a
		}
		contour[i] = b
	}
	return contour
}

type sincOscillator struct {
	phase float64
}

// render produces n carrier samples using the extracted sinc wavetable with
// linear interpolation; freqs holds one frequency per sample.
func (o *sincOscillator) render(freqs []float64, out []float64) {
	const size = float64(len(sincWavetable))
	for i := range out {
		o.phase = math.Mod(o.phase+freqs[i]*size/vectorAudioRate, size)
		idx := int(o.phase)
		frac := o.phase - float64(idx)
		next := (idx + 1) % len(sincWavetable)
		out[i] = float64(sincWavetable[idx])*(1-frac) + float64(sincWavetable[next])*frac
	}
}

type sawOscillator struct {
	phase float64
}

// render produces n samples of a polyBLEP band-limited sawtooth; freqs holds
// one frequency per sample. The real Vector voice has strong harmonics up to
// 6 kHz, which the clipped-sinc wavetable (band-limited by construction to
// ~7 harmonics) could never produce — the firmware's wavetable oscillator is
// driven as a saw, and polyBLEP is the clean equivalent.
func (o *sawOscillator) render(freqs []float64, out []float64) {
	for i := range out {
		f := freqs[i]
		dt := f / vocoderRate
		o.phase += dt
		if o.phase >= 1 {
			o.phase -= 1
		}
		t := o.phase
		v := 2*t - 1
		switch {
		case t < dt:
			u := t / dt
			v -= 2*u - u*u - 1
		case t > 1-dt:
			u := (t - 1) / dt
			v -= u*u + 2*u + 1
		}
		out[i] = v * 0.5
	}
}

type lowpassBiquad struct {
	b0, b1, b2 float64
	a1, a2     float64
	x1, x2     float64
	y1, y2     float64
}

func butterLP(sampleRate int, cutoff float64) lowpassBiquad {
	w := 2 * math.Pi * cutoff / float64(sampleRate)
	q := math.Sqrt2 / 2
	alpha := math.Sin(w) / (2 * q)
	a0 := 1 + alpha
	return lowpassBiquad{
		b0: (1 - math.Cos(w)) / 2 / a0,
		b1: (1 - math.Cos(w)) / a0,
		b2: (1 - math.Cos(w)) / 2 / a0,
		a1: -2 * math.Cos(w) / a0,
		a2: (1 - alpha) / a0,
	}
}

func (f *lowpassBiquad) process(x float64) float64 {
	y := f.b0*x + f.b1*f.x1 + f.b2*f.x2 - f.a1*f.y1 - f.a2*f.y2
	f.x2, f.x1 = f.x1, x
	f.y2, f.y1 = f.y1, y
	return y
}

// vectorVocodePCM reproduces Vector's voice: an STFT vocoder whose carrier is
// a band-limited sawtooth following the speech pitch, shaped by the firmware's
// 8-band carrier EQ. Processing runs at 22050 Hz like Vector's audio engine
// (the Acapela voices are 22 kHz); this also places the firmware's EQ bands at
// their true frequencies — at 16 kHz the passband lands an octave too low and
// the result sounds like a 2002 feature phone.
//
// carrierHz is kept as a pitch anchor: the tracked contour is scaled by
// carrierHz/180 before the [100, 260] Hz clamp, so lower values deepen the
// robot and higher values brighten it. wetMix (0..1) maps onto the wet/dry
// balance, from a dry-forward +(-6 dB) blend to an almost fully vocoded
// (+24 dB) output; 0 returns the input untouched.
func vectorVocodePCM(input []int16, carrierHz, wetMix float64) []int16 {
	if len(input) <= vocoderFrame {
		return append([]int16(nil), input...)
	}
	wetMix = math.Max(0, math.Min(1, wetMix))
	if wetMix == 0 {
		return append([]int16(nil), input...)
	}
	pitchScale := math.Max(0.5, math.Min(1.6, carrierHz/180))
	wetDryDb := wetMix*30 - 6

	// Live-tunable knobs, exposed as addon environment variables.
	gamma := envFloat("VECTOR_TTS_VOCODER_GAMMA", 1.35)
	gamma = math.Max(1, math.Min(2.5, gamma))
	gateThreshold := envFloat("VECTOR_TTS_VOCODER_GATE", 0.008)
	gateThreshold = math.Max(0, math.Min(0.05, gateThreshold))
	const gateFloor = 0.05
	// No low-pass by default: the real Vector voice keeps strong 2-6 kHz
	// content, and the earlier 2.2 kHz filter made it sound like a 2002
	// feature phone.
	lpfHz := envFloat("VECTOR_TTS_VOCODER_LPF", 0)
	lpfHz = math.Max(0, math.Min(7000, lpfHz))

	// Upsample to the firmware's native 22.05 kHz for processing.
	rs := resamplePCM16(input, vectorAudioRate, vocoderRate)
	n := len(rs)
	dry := make([]float64, n)
	for i, s := range rs {
		dry[i] = float64(s) / 32768
	}
	contour := trackPitch(rs, vocoderRate)
	window := vocoderWindow()
	eq := carrierEQGains()

	nFrames := (n - vocoderFrame) / vocoderHop
	outLen := nFrames*vocoderHop + vocoderFrame
	wet := make([]float64, outLen)
	carrierHist := make([]float64, vocoderFrame)
	modBuf := make([]complex128, vocoderFrame)
	carBuf := make([]complex128, vocoderFrame)
	frameFreqs := make([]float64, vocoderHop)
	frameCarrier := make([]float64, vocoderHop)
	var osc sawOscillator
	current := 180.0
	target := 180.0
	step := 0.0
	glideLeft := 0
	gateGain := 0.0

	for frame := 0; frame < nFrames; frame++ {
		// Carrier frequency update: pitch-tracked, clamped, then glided over
		// one hop, like the firmware's per-RTPC glide.
		pitch := 0.0
		if frame < len(contour) {
			pitch = contour[frame]
		}
		if pitch > 0 {
			newTarget := math.Max(vocoderPitchMin, math.Min(vocoderPitchMax, pitch*pitchScale))
			if math.Abs(newTarget-target) > 1e-6 {
				target = newTarget
				step = (target - current) / vocoderGlide
				glideLeft = vocoderGlide
			}
		}
		for i := 0; i < vocoderHop; i++ {
			if glideLeft > 0 {
				current += step
				glideLeft--
			} else {
				current = target
			}
			frameFreqs[i] = current
		}
		osc.render(frameFreqs, frameCarrier)
		copy(carrierHist, carrierHist[vocoderHop:])
		copy(carrierHist[vocoderHop:], frameCarrier)

		// Modulator analysis.
		offset := frame * vocoderHop
		var frameEnergy float64
		for i := 0; i < vocoderFrame; i++ {
			v := dry[offset+i] * window[i]
			modBuf[i] = complex(v, 0)
			frameEnergy += dry[offset+i] * dry[offset+i]
		}
		fft1024(modBuf, false)
		frameRMS := math.Sqrt(frameEnergy / vocoderFrame)
		// Soft silence gate: fast open, slow close, never fully shut.
		gateTarget := 1.0
		if frameRMS < gateThreshold {
			gateTarget = gateFloor
		}
		if frame == 0 {
			gateGain = gateTarget
		} else if gateTarget > gateGain {
			gateGain = gateGain*0.3 + gateTarget*0.7
		} else {
			gateGain = gateGain*0.75 + gateTarget*0.25
		}
		peakMag := 0.0
		mags := make([]float64, vocoderFrame)
		for i := 0; i < vocoderFrame; i++ {
			mags[i] = math.Min(1e9, math.Hypot(real(modBuf[i]), imag(modBuf[i])))
			if mags[i] > peakMag {
				peakMag = mags[i]
			}
		}
		if gamma != 1.0 && peakMag > 1e-12 {
			inv := 1.0 / peakMag
			for i := range mags {
				mags[i] = math.Pow(mags[i]*inv, gamma) * peakMag
			}
		}
		if gateThreshold > 0 {
			for i := range mags {
				mags[i] *= gateGain
			}
		}

		// Carrier spectrum (unwindowed, as in the firmware), shaped by the
		// modulator envelope and the graphic EQ.
		for i := 0; i < vocoderFrame; i++ {
			carBuf[i] = complex(carrierHist[i], 0)
		}
		fft1024(carBuf, false)
		for i := 0; i < vocoderFrame; i++ {
			carBuf[i] *= complex(mags[i]*eq[i], 0)
		}
		fft1024(carBuf, true)
		scale := 1.0 / vocoderFrame
		for i := 0; i < vocoderFrame; i++ {
			wet[offset+i] += real(carBuf[i]) * scale * window[i]
		}
	}

	// Calibrate the wet layer so its RMS sits wetDryDb above the dry RMS over
	// the voiced region; the original achieves the same with fixed dB gains on
	// its own internal scaling.
	const dryGain = 0.8913 // firmware dry level: -1 dB
	var dryRMS, wetRMS float64
	var count int
	for i := vocoderFrame / 2; i < outLen && i < n; i++ {
		if math.Abs(dry[i]) > 0.02 {
			dryRMS += dry[i] * dry[i]
			wetRMS += wet[i] * wet[i]
			count++
		}
	}
	wetGain := 0.0
	if count > 100 && wetRMS > 1e-12 {
		wetGain = math.Sqrt(dryRMS/wetRMS) * math.Pow(10, wetDryDb/20) / dryGain
	}

	var lp1, lp2 lowpassBiquad
	if lpfHz > 0 {
		lp1 = butterLP(vocoderRate, lpfHz)
		lp2 = butterLP(vocoderRate, lpfHz)
	}
	peak := 0.0
	mixed := make([]float64, n)
	for i := 0; i < n; i++ {
		v := dryGain * dry[i]
		if i < outLen {
			v += wetGain * wet[i]
		}
		if lpfHz > 0 {
			v = lp2.process(lp1.process(v))
		}
		mixed[i] = v
		if math.Abs(v) > peak {
			peak = math.Abs(v)
		}
	}
	// Safety ceiling only: scale the whole utterance once, no pumping.
	ceiling := math.Pow(10, -2.0/20.0)
	gain := 1.0
	if peak > ceiling {
		gain = ceiling / peak
	}
	// Back to the robot's 16 kHz streaming rate.
	out := make([]int16, len(input))
	ratio := float64(n-1) / float64(len(input))
	for i := range out {
		position := float64(i) * ratio
		lo := int(position)
		if lo >= n-1 {
			out[i] = int16(math.Round(mixed[n-1] * gain * 32767))
			continue
		}
		fraction := position - float64(lo)
		v := mixed[lo]*(1-fraction) + mixed[lo+1]*fraction
		out[i] = int16(math.Round(v * gain * 32767))
	}
	return out
}
