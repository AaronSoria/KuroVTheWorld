# 07 — Audio

El sonido es la mitad del impacto de un golpe. Un puñetazo con un «thud» seco y
un hitstop de 4 ticks se siente completamente distinto sin audio. Este capítulo
cubre lo mínimo para música, efectos, mezcla y problemas típicos.

## 1. El contexto de audio

Ebitengine usa un único contexto de audio con una frecuencia de muestreo:

```go
const sampleRate = 48000

func main() {
	audio.NewContext(sampleRate)
	// ...
}
```

Reglas:

- Llama a `audio.NewContext` **una sola vez**, al arrancar. Si ya existe, entra
  en pánico.
- 44100 o 48000 son valores habituales. Elige uno y convierte todos los assets a
  esa frecuencia; el motor puede remuestrear, pero es mejor controlarlo.
- `audio.CurrentContext()` devuelve el contexto actual si necesitas crearlo en
  otro sitio, pero no lo uses para crear dos.
- En navegador, el audio puede requerir una interacción del usuario. Diseña la
  pantalla de título para que la primera tecla o clic también «desbloquee» el
  audio.

## 2. Música: streaming o bucle

### Streaming

Para pistas largas, decodifica desde un lector y deja que el motor lea por
trozos:

```go
f, err := os.Open("assets/audio/stage1.ogg")
if err != nil {
	return err
}
defer f.Close()

stream, err := vorbis.DecodeWithSampleRate(sampleRate, f)
if err != nil {
	return err
}
music, err := audio.NewPlayer(ctx, stream)
if err != nil {
	return err
}
music.SetVolume(0.6)
music.Play()
```

Para MP3, `mp3.DecodeWithSampleRate`; para WAV, `wav.DecodeWithSampleRate`. Los
tres devuelven un `*Stream` que entrega PCM de 16 bits con signo, little-endian y
2 canales, e implementa `io.ReadSeeker`. `Seek` solo funciona si la fuente
original es `io.Seeker`; si no, `Rewind` devolverá error.

### Bucle sin silencio

Reiniciar el reproductor al terminar puede dejar un hueco audible. Para un bucle
perfecto:

1. Decodifica el archivo a PCM.
2. Usa `audio.NewInfiniteLoop` con la longitud en bytes del PCM.
3. Crea el reproductor con ese stream.

`NewInfiniteLoop` espera PCM entero de 16 bits little-endian, estéreo. Si el
archivo tiene un intro y luego un bucle, usa `NewInfiniteLoopWithIntro`.

Para el primer prototipo, reiniciar al terminar es aceptable:

```go
func (m *Music) Update() {
	if m.player == nil || m.player.IsPlaying() {
		return
	}
	if err := m.player.Rewind(); err == nil {
		m.player.Play()
	}
}
```

### Música que cambia

- Título, nivel, jefe y game over pueden ser pistas distintas.
- Haz *crossfade* bajando el volumen de una y subiendo el de otra durante ~1 s,
  o corta en seco si el estilo lo permite.
- No crees un reproductor nuevo cada vez que cambias de pista sin pausar el
  anterior: se solapan.

## 3. Efectos de sonido

### Cargar una vez

```go
//go:embed assets/audio/hit.wav
var hitWAV []byte

func loadSFX(ctx *audio.Context, data []byte) (*audio.Player, error) {
	stream, err := wav.DecodeWithSampleRate(ctx.SampleRate(), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	// Si el stream es corto, puedes materializarlo en bytes PCM.
	return ctx.NewPlayer(stream)
}
```

Para efectos cortos, `ctx.NewPlayerFromBytes(pcm)` es cómodo: no falla y crea un
`*Player` sobre un `bytes.Reader` (que sí es *seeker*, así que `Rewind` funciona).

### Reproducir el mismo sonido varias veces

Un `*Player` reproduce un stream. Para golpes que se solapan necesitas varias
voces:

```go
type SFX struct {
	players []*audio.Player
	next    int
	volume  float64
}

func NewSFX(ctx *audio.Context, pcm []byte, voices int, volume float64) *SFX {
	s := &SFX{volume: volume}
	for i := 0; i < voices; i++ {
		p := ctx.NewPlayerFromBytes(pcm)
		p.SetVolume(volume)
		s.players = append(s.players, p)
	}
	return s
}

func (s *SFX) Play() {
	// Busca una voz libre.
	for _, p := range s.players {
		if p.IsPlaying() {
			continue
		}
		_ = p.Rewind()
		p.Play()
		return
	}
	// Todas ocupadas: roba la siguiente (aceptable para SFX cortos).
	p := s.players[s.next]
	s.next = (s.next + 1) % len(s.players)
	_ = p.Rewind()
	p.Play()
}

func (s *SFX) SetVolume(v float64) {
	s.volume = v
	for _, p := range s.players {
		p.SetVolume(v)
	}
}
```

Recomendaciones:

- 4–8 voces por efecto es suficiente. Más voces no se distinguen y saturan.
- Un sonido de impacto distinto para golpe ligero, pesado, bloqueo y caída.
- Aleatoriza ligeramente el volumen (0.9–1.0) para evitar repetición. No cambies
  el pitch en tiempo real salvo que tengas un sistema de resampling; prepara 2–3
  variantes pregrabadas.
- No crees un `*audio.Player` por golpe en `Update`: es una asignación y un
  recurso que puede no liberarse cuando esperas.

### Duración y prioridad

Los sonidos de impacto deben ser cortos (60–200 ms). Si duran más, se solapan y
emborronan. Reserva sonidos largos para muertes, jefes y ambiente.

## 4. Mezcla y volúmenes

No hay un mezclador global; el volumen se aplica por reproductor. Mantén una
jerarquía:

```go
type AudioSettings struct {
	Master float64 // 0..1
	Music  float64
	SFX    float64
}

func (a AudioSettings) MusicVolume() float64 { return a.Master * a.Music }
func (a AudioSettings) SFXVolume() float64   { return a.Master * a.SFX }
```

Al cambiar un ajuste, recorre los reproductores y aplica el volumen resultante.
No guardes el volumen final en cada player como si fuera estado independiente;
si el usuario cambia el master, todos deben actualizarse.

### Silenciar al perder foco

```go
if !ebiten.IsFocused() {
	// Baja la música o pausa el juego.
}
```

No pauses el contexto de audio global: afecta a todo y puede ser difícil de
recuperar. Baja el volumen de la música y deja que el juego se pause.

## 5. Latencia y buffer

El tamaño del buffer afecta al retraso entre `Play()` y el sonido:

```go
p.SetBufferSize(50 * time.Millisecond)
```

- Un buffer grande es robusto pero añade latencia; en un juego de acción se nota.
- Un buffer pequeño reduce el retraso pero puede producir cortes si el sistema
  está cargado.
- Ajusta antes de `Play`. No cambies el buffer cada frame.
- Si el juego va a 60 TPS, 16 ms por tick; un buffer de 50 ms son ~3 ticks. Es un
  compromiso razonable.

## 6. Sincronización con el combate

Dispara el sonido en el tick en que la hitbox conecta, no en `Draw`:

```go
func (w *World) applyHit(h Hit) {
	// ... daño y estado ...
	if h.Move.HitSound != "" {
		w.Audio.Play(h.Move.HitSound)
	}
}
```

Si el sonido va un tick tarde, el golpe se siente «blando». Si va un tick antes,
se siente «mágico». El oído es muy sensible a esto; pruébalo con los ojos
cerrados.

Para el bloqueo, usa un sonido metálico distinto y más corto. Para el whiff
(ataque al aire), un sonido de roce; ayuda a que el jugador sepa que falló.

## 7. Problemas frecuentes

| Síntoma | Causa probable |
|---|---|
| `panic: audio: context is already created` | `NewContext` llamado dos veces |
| El sonido se corta al reiniciar | Se reutiliza un player que sigue sonando |
| Los golpes suenan todos igual | Una sola voz o falta de variantes |
| La música se solapa | No se pausó la pista anterior |
| Latencia alta | Buffer grande o sonido disparado en `Draw` |
| El juego va a tirones al reproducir | Se decodifica o se crea un player por golpe |
| En navegador no suena al principio | Falta interacción del usuario |
| El volumen no cambia | Se aplicó solo al crear el player |

## 8. Ejercicio del capítulo

1. Crea el contexto de audio a 48000 Hz.
2. Carga una música de fondo y haz que se repita al terminar.
3. Carga tres efectos: golpe ligero, golpe pesado y bloqueo.
4. Implementa un pool de 4 voces por efecto y dispara desde el combate.
5. Añade un ajuste de volumen master/música/SFX y aplica los cambios en caliente.
6. Comprueba que el sonido coincide con el tick activo usando el debug.
7. Pausa la música al perder foco y reanúdala al recuperarlo sin reiniciar la
   pista.

Criterio de aceptación: puedes golpear a dos enemigos seguidos y oír dos impactos
sin que se corte el primero, y la música no se solapa al cambiar de escena.

## 9. Referencias

- `audio/audio.go`: `Context`, `Player`, `NewInfiniteLoop`.
- `audio/mp3`, `audio/vorbis`, `audio/wav`: decodificadores.
- `examples/audio` y `examples/wav` para reproductores.
- `audio.NewContext` y `SetBufferSize` en la documentación del paquete.
