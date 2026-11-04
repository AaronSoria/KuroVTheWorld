# Apéndice — Datos, API y glosario

Este apéndice es para consulta rápida. No sustituye a `go doc`, pero reúne las
firmas y esquemas que más vas a usar.

## 1. Chuleta de API

### Bucle y ventana

```go
func main() {
	ebiten.SetWindowSize(960, 540)
	ebiten.SetWindowTitle("Mi beat 'em up")
	ebiten.SetTPS(60) // opcional; 60 es el valor por defecto
	if err := ebiten.RunGame(&Game{}); err != nil {
		log.Fatal(err)
	}
}
```

```go
type Game interface {
	Update() error
	Draw(screen *ebiten.Image)
	Layout(outsideWidth, outsideHeight int) (int, int)
}
```

```go
ebiten.TPS()            // ticks objetivo por segundo
ebiten.ActualTPS()      // ticks reales
ebiten.ActualFPS()      // frames reales
ebiten.Tick()           // contador global de ticks
ebiten.IsFocused()      // ¿la ventana tiene foco?
```

### Imágenes y dibujo

```go
img := ebiten.NewImage(w, h)
img.Fill(color.RGBA{0x20, 0x20, 0x30, 0xff})
img.Clear()

tex := ebiten.NewImageFromImage(stdImage)
sub := sheet.SubImage(image.Rect(x, y, x+w, y+h)).(*ebiten.Image)

op := &ebiten.DrawImageOptions{}
op.GeoM.Scale(sx, sy)
op.GeoM.Translate(x, y)
op.ColorScale.ScaleAlpha(0.5)
op.Filter = ebiten.FilterNearest // valor por defecto
op.Blend = ebiten.BlendSourceOver
screen.DrawImage(sub, op)
```

`image.Rect(x0, y0, x1, y1)` usa el máximo exclusivo: el ancho es `x1-x0`.

```go
op.ColorScale.Set(1.2, 1.2, 1.2, 1.0) // multiplica canales
```

### GeoM

```go
op.GeoM.Reset()
op.GeoM.Translate(x, y)
op.GeoM.Scale(sx, sy)
op.GeoM.Rotate(radianes) // sentido horario
op.GeoM.Concat(otra)
```

El orden de llamadas es el orden de aplicación: `Scale` y luego `Translate`
escala primero y traslada después. Para dibujar anclado por los pies:

```go
op.GeoM.Translate(-w/2, -h)
op.GeoM.Translate(screenX, screenY)
```

### Entrada

```go
ebiten.IsKeyPressed(ebiten.KeyA)
inpututil.IsKeyJustPressed(ebiten.KeySpace)
inpututil.IsKeyJustReleased(ebiten.KeySpace)
inpututil.KeyPressDuration(ebiten.KeySpace) // ticks

x, y := ebiten.CursorPosition()
ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
```

### Gamepad

```go
for _, id := range ebiten.AppendGamepadIDs(nil) {
	h := ebiten.StandardGamepadAxisValue(
		id, ebiten.StandardGamepadAxisLeftStickHorizontal)
	v := ebiten.StandardGamepadAxisValue(
		id, ebiten.StandardGamepadAxisLeftStickVertical)
	_, _ = h, v

	a := ebiten.IsStandardGamepadButtonPressed(
		id, ebiten.StandardGamepadButtonRightBottom)
	justA := inpututil.IsStandardGamepadButtonJustPressed(
		id, ebiten.StandardGamepadButtonRightBottom)
	_, _ = a, justA
}
```

Aplica zona muerta al stick y comprueba los IDs cada tick.

### Texto (`text/v2`)

```go
src, err := text.NewGoTextFaceSource(bytes.NewReader(ttf))
if err != nil {
	return err
}
face := &text.GoTextFace{Source: src, Size: 16}

op := &text.DrawOptions{}
op.GeoM.Translate(x, y) // esquina superior izquierda
op.ColorScale.ScaleWithColor(color.White)
op.PrimaryAlign = text.AlignCenter
op.SecondaryAlign = text.AlignCenter
text.Draw(dst, "HOLA", face, op)

w, h := text.Measure("HOLA", face, 0)
text.CacheGlyphs(face, "0123456789")
```

La v1 (`github.com/hajimehoshi/ebiten/v2/text`) usa `font.Face` y posiciones de
línea base; no mezcles ambas APIs.

### Audio

```go
ctx := audio.NewContext(48000) // una sola vez
p := ctx.NewPlayerFromBytes(pcm) // no falla; bytes debe ser PCM 16-bit estéreo
p.SetVolume(0.8)
p.Play()
if !p.IsPlaying() {
	_ = p.Rewind()
	p.Play()
}
p.Pause()
p.PauseAndStopReading() // limpieza; Close está deprecado
```

```go
stream, err := wav.DecodeWithSampleRate(48000, reader)
p, err := ctx.NewPlayer(stream)
```

### Vector

```go
vector.DrawFilledRect(dst, x, y, w, h float32, clr, antialias bool)
vector.StrokeRect(dst, x, y, w, h, strokeWidth float32, clr, antialias bool)
vector.DrawFilledCircle(dst, cx, cy, r float32, clr, antialias bool)
```

Las coordenadas de `vector` son `float32`; las de `GeoM` son `float64`.

### Debug

```go
ebitenutil.DebugPrint(screen, "texto")
ebitenutil.DebugPrintAt(screen, "texto", x, y)
```

```bash
go run -tags=ebitenginedebug ./cmd/game
```

## 2. Esquema de un movimiento

```json
{
  "moves": {
    "jab": {
      "name": "jab",
      "anim": "attack_1",
      "startup": 4,
      "active": 2,
      "recovery": 8,
      "damage": 6,
      "hitstun": 14,
      "blockstun": 8,
      "hitstop": 4,
      "knockbackX": 1.5,
      "knockbackY": 0,
      "hitbox": {
        "offsetX": 18,
        "offsetY": 0,
        "width": 24,
        "depth": 18,
        "zMin": 20,
        "zMax": 50
      },
      "cancelFrom": 6,
      "next": ["jab2"]
    }
  }
}
```

Invariantes que conviene validar al cargar:

- `startup >= 0`, `active > 0`, `recovery >= 0`.
- `damage >= 0`, `hitstun >= 0`, `hitstop >= 0`.
- `cancelFrom` dentro de `[0, total]`.
- `next` solo referencia movimientos existentes.
- `hitbox.width > 0`, `hitbox.depth > 0`, `zMax > zMin`.
- Un movimiento cancelable debe tener `next` no vacío.

## 3. Esquema de un nivel

```json
{
  "name": "street01",
  "music": "stage1",
  "bounds": { "minX": 0, "maxX": 3200, "minY": 180, "maxY": 300 },
  "cameraBounds": { "minX": 0, "maxX": 3200, "minY": 0, "maxY": 270 },
  "exitX": 3100,
  "waves": [
    {
      "id": "w1",
      "trigger": { "playerX": 300, "afterWave": "" },
      "lockCamera": true,
      "cameraLockX": 240,
      "maxAlive": 2,
      "spawnInterval": 45,
      "nextWhenClear": true,
      "spawns": [
        { "kind": "thug", "x": 560, "y": 230 },
        { "kind": "thug", "x": 620, "y": 270 },
        { "kind": "bruiser", "x": 700, "y": 250 }
      ]
    }
  ]
}
```

Validación mínima:

- `maxX > minX`, `maxY > minY`.
- `cameraBounds` contiene o interseca los bounds del juego.
- Cada spawn está dentro de `bounds` y no dentro de una pared.
- IDs de oleada únicos.
- `afterWave` referencia una oleada anterior existente.
- `exitX` está dentro del nivel.

## 4. Esquema de guardado

```json
{
  "version": 2,
  "settings": {
    "masterVolume": 0.8,
    "musicVolume": 0.6,
    "sfxVolume": 0.9,
    "fullscreen": false,
    "keybinds": { "attack": 74, "heavy": 75, "jump": 32, "pause": 27 },
    "language": "es",
    "shake": true,
    "flashes": true
  },
  "progress": {
    "unlockedLevel": 2,
    "bestScores": { "street01": 12500 },
    "totalKills": 42,
    "cleared": false
  }
}
```

Reglas: `version` en la raíz, validar rangos al cargar, escribir de forma atómica,
y conservar una copia si el archivo está corrupto.

## 5. Tabla de frame data de ejemplo

| Movimiento | Startup | Active | Recovery | Total | Daño | Hitstun | Blockstun | Hitstop | Cancel desde |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| Jab | 4 | 2 | 8 | 14 | 6 | 14 | 8 | 4 | 6 |
| Jab 2 | 6 | 2 | 10 | 18 | 8 | 16 | 10 | 5 | 10 |
| Finisher | 10 | 3 | 18 | 31 | 14 | 24 | 14 | 8 | — |
| Patada baja | 7 | 3 | 12 | 22 | 9 | 18 | 9 | 6 | 10 |
| Golpe fuerte | 14 | 3 | 20 | 37 | 18 | 28 | 16 | 9 | — |

Ventaja tras conectar = `hitstun - recovery` (para el atacante). Si es positiva,
puede actuar antes que la víctima. Compruébalo con la simulación, no solo con la
tabla: la cancelación y el hitstop también influyen.

## 6. Errores de compilación frecuentes

| Error | Causa / solución |
|---|---|
| `cannot use img (variable of type image.Image) as *ebiten.Image` | `SubImage` devuelve `image.Image`; haz `.(*ebiten.Image)` |
| `undefined: color` | Falta `import "image/color"` |
| `cannot use x (variable of type int) as float64` | `GeoM` usa `float64`; convierte con `float64(x)` |
| `cannot use x (variable of type float64) as float32` | `vector` usa `float32`; convierte con `float32(x)` |
| `declared and not used` | Go no permite variables sin usar; elimínalas o usa `_` |
| `screen.DrawImage undefined` | `screen` puede estar sombreado por un parámetro; renombra |
| `panic: image is disposed` | Usaste una imagen después de `Dispose` o de cambiar de escena |
| `panic: audio: context is already created` | `audio.NewContext` llamado dos veces |
| `text.Draw` con tipos incompatibles | Mezclaste `text` v1 y `text/v2` |

## 7. Glosario

| Término | Significado |
|---|---|
| **Beat 'em up** | Juego de lucha con scroll lateral contra grupos de enemigos |
| **Vertical slice** | Rebanada pequeña pero completa del juego, de principio a fin |
| **Hitbox** | Caja que representa el alcance de un ataque |
| **Hurtbox** | Caja que representa la zona vulnerable de un personaje |
| **Frame data** | Ticks de startup, active, recovery y sus efectos |
| **Startup** | Preparación antes de que la hitbox esté activa |
| **Active** | Ticks en los que el ataque puede golpear |
| **Recovery** | Ticks de recuperación tras el ataque |
| **Hitstun** | Ticks que la víctima no puede actuar tras recibir un golpe |
| **Blockstun** | Ticks que la víctima no puede actuar tras bloquear |
| **Hitstop** | Congelación breve al conectar, para dar peso |
| **Knockback** | Impulso que aleja a la víctima |
| **Juggle** | Mantener a un enemigo en el aire con varios golpes |
| **I-frames** | Ticks de invulnerabilidad |
| **Cancel** | Interrumpir la recuperación de un ataque con otro |
| **Whiff** | Ataque que no conecta |
| **Telegraph** | Señal visual previa a un ataque enemigo |
| **Token** | Permiso global para que un enemigo ataque; limita la presión |
| **TTK** | Tiempo para matar a un enemigo |
| **Depth / Y-sorting** | Profundidad del suelo y orden de dibujo por ella |
| **Parallax** | Capas de fondo que se mueven a distinta velocidad |
| **Pool** | Reutilización de objetos para evitar asignaciones |
| **Game feel** | Sensación táctil: peso, respuesta, feedback |
| **Determinismo** | Misma entrada y semilla producen el mismo resultado |

## 8. Referencias y siguientes pasos

Documentación y código:

- <https://ebitengine.org/>
- <https://pkg.go.dev/github.com/hajimehoshi/ebiten/v2>
- <https://github.com/hajimehoshi/ebiten/tree/main/examples>
- `examples/platformer`, `examples/animation`, `examples/camera`,
  `examples/2048`, `examples/audio`, `examples/text`, `examples/blocks`.
- `skills/efficient-ebitengine-rendering/SKILL.md`
- `skills/avoiding-blurry-ebitengine-rendering/SKILL.md`
- `skills/writing-kage-shaders/SKILL.md` si más adelante quieres efectos.

Siguientes pasos sugeridos, en orden:

1. Termina el vertical slice del capítulo 10.
2. Añade un segundo nivel con un jefe y una mecánica nueva.
3. Añade armas recogibles o agarres, no ambos a la vez.
4. Si quieres cooperativo, rediseña cámara, tokens y estados desde el principio;
   no lo injertes al final.
5. Publica una versión jugable y observa a personas reales jugarla.
6. Solo entonces plantea más contenido, modos y desbloqueos.

La documentación oficial es la referencia final para la API. Esta guía es el
mapa; el código lo escribes tú.
