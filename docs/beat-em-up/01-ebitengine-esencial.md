# 01 — Ebitengine esencial

Antes de diseñar un beat 'em up hay que entender el contrato entre tu juego y el
motor. Ebitengine no es un motor de escenas ni un editor: es una capa fina sobre
gráficos, entrada y audio que llama a tu código en un bucle. Tú eres el dueño del
estado del juego.

## 1. El contrato: `ebiten.Game`

Un juego es cualquier tipo que implemente tres métodos:

```go
type Game interface {
	Update() error
	Draw(screen *ebiten.Image)
	Layout(outsideWidth, outsideHeight int) (int, int)
}
```

```go
type Game struct {
	ticks int
}

func (g *Game) Update() error {
	g.ticks++
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Aquí solo se dibuja. No se modifica el estado del juego.
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 480, 270 // resolución lógica del juego
}

func main() {
	if err := ebiten.RunGame(&Game{}); err != nil {
		log.Fatal(err)
	}
}
```

Puntos importantes:

- `Update` actualiza el estado. Es el único sitio donde debe cambiar la lógica.
- `Draw` recibe una imagen que representa la pantalla lógica. Se puede llamar más
  o menos veces que `Update`, y nunca debes asumir una relación 1:1.
- `Layout` devuelve el tamaño lógico de la pantalla. Se llama al arrancar y
  cuando cambia el tamaño de la ventana; no es un sitio para cargar recursos.
- Si `Update` devuelve un error, el bucle termina y `RunGame` devuelve ese error.
  Para una salida limpia (por ejemplo, desde «Salir» en el menú), devuelve
  `ebiten.Termination`: el bucle se detiene y `RunGame` no devuelve error.

### Ticks, frames y por qué no usar `time.Now`

Por defecto Ebitengine ejecuta `Update` 60 veces por segundo (`ebiten.TPS()`), y
`Draw` tantas veces como permita la pantalla y VSync. A esto se le llama *fixed
timestep*: la lógica avanza en pasos discretos y reproducibles.

```go
const ticksPerSecond = 60

func (g *Game) Update() error {
	g.ticks++
	return nil
}
```

No uses `time.Now()` ni `time.Sleep` para animar. Si necesitas segundos:

```go
dt := 1.0 / float64(ebiten.TPS())
```

Pero para un beat 'em up es mejor pensar en ticks: «este golpe tarda 4 ticks en
activar, 2 en golpear y 8 en recuperarse». Es más fácil de ajustar, de testear y
de reproducir. Puedes cambiar `ebiten.SetTPS(60)` si necesitas otra cadencia, pero
60 es la referencia y muchos juegos del género están diseñados a 60.

`ebiten.Tick()` devuelve un contador global que avanza una vez por `Update`.
Úsalo para animaciones puramente visuales o para temporizadores que no formen
parte de la simulación; para el combate, lleva tus propios contadores.

### El bucle completo, en orden

En cada tick, Ebitengine hace algo parecido a esto:

1. Actualiza el estado de teclado, ratón, gamepads y táctil.
2. Llama a `Update`.
3. Si toca dibujar, llama a `Draw` con la pantalla lógica.
4. Presenta la imagen en la ventana.

Consecuencias:

- El input que lees en `Update` es el del tick actual.
- Un `Draw` puede ocurrir después de varios `Update`, o entre dos.
- No guardes punteros a la pantalla ni a imágenes temporales fuera de `Draw`.

## 2. Resolución lógica, ventana y píxeles

Separa siempre dos tamaños:

- **Tamaño lógico**: el que devuelve `Layout`. Define la cámara, el HUD y las
  proporciones. Ejemplo: 480×270 (16:9) o 320×180.
- **Tamaño de ventana**: píxeles reales en pantalla. Ejemplo: 960×540 (×2).

```go
const (
	screenWidth  = 480
	screenHeight = 270
)

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	ebiten.SetWindowSize(screenWidth*2, screenHeight*2)
	ebiten.SetWindowTitle("Mi beat 'em up")
	// ...
}
```

Para pixel art:

- Usa un `Layout` fijo y escala la ventana por múltiplos enteros (×2, ×3, ×4).
- Deja `Filter` en `ebiten.FilterNearest` (es el valor por defecto).
- Redondea posiciones de cámara y de sprite a píxeles enteros al dibujar, o
  verás temblor y píxeles borrosos.
- No dibujes a resoluciones intermedias y escales después sin necesidad.

Si quieres una ventana redimensionable, `ebiten.SetWindowResizingMode` permite
distintos modos. Decide pronto si el juego admite resize; cambia cómo calculas la
escala y el letterboxing.

## 3. Coordenadas y `GeoM`

El origen `(0, 0)` está en la esquina superior izquierda. `X` crece a la derecha
e `Y` hacia abajo. Un píxel lógico ocupa el cuadrado `[x, x+1) × [y, y+1)`.

Para dibujar se usa una matriz `ebiten.GeoM`. Las transformaciones se aplican en
el orden en que se llaman:

```go
op := &ebiten.DrawImageOptions{}
op.GeoM.Scale(2, 2)      // 1. escala
op.GeoM.Translate(100, 50) // 2. traslada
screen.DrawImage(sprite, op)
```

El resultado es «escala el sprite y luego colócalo en (100, 50)». Si inviertes el
orden, la traslación también se escala.

Las operaciones más usadas:

```go
op.GeoM.Reset()                 // identidad
op.GeoM.Translate(x, y)         // mover
op.GeoM.Scale(sx, sy)           // escalar (negativo = espejo)
op.GeoM.Rotate(radianes)        // rotar en sentido horario
op.GeoM.Concat(otra)            // componer otra matriz
```

Para dibujar un sprite centrado en los pies (lo usaremos en el capítulo 03):

```go
w := float64(frame.Bounds().Dx())
h := float64(frame.Bounds().Dy())

op := &ebiten.DrawImageOptions{}
op.GeoM.Translate(-w/2, -h) // origen en el centro de los pies
op.GeoM.Scale(sx, sy)       // escala respecto a ese origen
op.GeoM.Translate(screenX, screenY)
screen.DrawImage(frame, op)
```

`GeoM` no modifica la imagen: solo transforma cómo se proyecta al dibujarla.

## 4. Imágenes

### Crear y cargar

```go
// Imagen vacía (útil para capas y efectos).
layer := ebiten.NewImage(480, 270)

// Desde un image.Image de la biblioteca estándar.
img, _, err := image.Decode(reader)
if err != nil {
	return err
}
tex := ebiten.NewImageFromImage(img)
```

En un juego real, carga una sola vez al arrancar, no en cada `Draw`:

```go
//go:embed assets/player.png
var playerPNG []byte

func loadImage(data []byte) (*ebiten.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return ebiten.NewImageFromImage(img), nil
}
```

`//go:embed` incrusta los assets en el binario. Es cómodo para distribuir y evita
rutas relativas frágiles. Para assets grandes o mods, carga desde disco.

### Subimágenes y sprite sheets

Una subimagen comparte píxeles con la original y se dibuja como una imagen
pequeña: su esquina superior izquierda es `(0, 0)` para fines de dibujo, aunque
`Bounds().Min` no sea cero.

```go
sheet := loadSheet()
frameRect := image.Rect(32*3, 0, 32*4, 48)
frame := sheet.SubImage(frameRect).(*ebiten.Image)

op := &ebiten.DrawImageOptions{}
screen.DrawImage(frame, op) // dibuja el frame en (0, 0)
```

No uses `Bounds().Min` para posicionar el frame; usa el ancho y alto
(`Bounds().Dx()`, `Bounds().Dy()`) y tu propia posición.

### Dibujar

```go
op := &ebiten.DrawImageOptions{}
op.GeoM.Translate(10, 20)
op.ColorScale.ScaleAlpha(0.5) // semitransparente
op.Filter = ebiten.FilterNearest
screen.DrawImage(sprite, op)
```

- `DrawImage` mezcla la fuente sobre el destino; no borra el fondo.
- La pantalla se limpia automáticamente cada frame por defecto. Si necesitas
  acumular, usa una imagen aparte o `ebiten.SetScreenClearedEveryFrame(false)`.
- `ColorScale` multiplica los canales. Es la forma barata de tintar o atenuar.
- `ColorScale` se aplica a colores con alfa premultiplicado; para operaciones de
  color más complejas, `colorm` o un shader.

### Render offscreen

```go
if g.worldLayer == nil {
	g.worldLayer = ebiten.NewImage(screenWidth, screenHeight)
}
g.worldLayer.Clear()
drawWorld(g.worldLayer)
screen.DrawImage(g.worldLayer, nil)
```

Úsalo cuando necesites un efecto de capa completa (desenfoque, oscurecer el
mundo tras un menú, capturas). No crees una imagen offscreen por entidad ni por
frame; es una de las causas más comunes de caídas de rendimiento.

## 5. Entrada

### Teclado y ratón

```go
if ebiten.IsKeyPressed(ebiten.KeyA) {
	// mantener
}
if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
	// solo en el primer Update tras pulsar
}
if inpututil.IsKeyJustReleased(ebiten.KeySpace) {
	// solo en el primer Update tras soltar
}
if d := inpututil.KeyPressDuration(ebiten.KeySpace); d > 0 {
	// duración en ticks
}
```

`IsKeyJustPressed` está pensado para `Update`. No lo llames desde `Draw` ni desde
una goroutine.

Ratón:

```go
x, y := ebiten.CursorPosition()
if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) { /* ... */ }
if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) { /* ... */ }
```

### Gamepads

```go
ids := ebiten.AppendGamepadIDs(nil)
for _, id := range ids {
	h := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
	v := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
	_ = h
	_ = v

	if ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightBottom) {
		// botón inferior (A en Xbox, cruz en PlayStation)
	}
	if inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonRightBottom) {
		// recién pulsado
	}
}
```

El mapeo estándar evita depender de la marca del mando. Comprueba siempre los IDs
en cada `Update`: los mandos se conectan y desconectan en caliente. Aplica una
zona muerta al stick (por ejemplo, 0.2) para evitar deriva.

### Zona muerta y normalización

```go
func applyDeadzone(v, deadzone float64) float64 {
	if math.Abs(v) < deadzone {
		return 0
	}
	// Reescala el resto para que el rango útil siga siendo [-1, 1].
	sign := 1.0
	if v < 0 {
		sign = -1
	}
	return sign * (math.Abs(v) - deadzone) / (1 - deadzone)
}
```

### Buffer de input

En un beat 'em up, pulsar un botón unos ticks antes de poder actuar debe contar.
No lo implementes con `IsKeyJustPressed` en el momento del ataque; guarda la
intención:

```go
type InputBuffer struct {
	attackTick int64 // último tick en que se pulsó atacar
	window     int64 // ticks de validez
}

func (b *InputBuffer) Pressed(now int64) {
	b.attackTick = now
}

func (b *InputBuffer) ConsumeAttack(now int64) bool {
	if now-b.attackTick <= b.window {
		b.attackTick = -1000 // consumido
		return true
	}
	return false
}
```

Este patrón se desarrolla en el capítulo 05.

## 6. Ventana, foco y pausa

- `ebiten.IsFocused()` indica si la ventana tiene foco. Es habitual pausar el
  juego o bajar el volumen cuando se pierde.
- `ebiten.SetCursorMode` permite ocultar el cursor.
- `ebiten.SetWindowTitle`, `SetWindowIcon`, `SetWindowSize`.
- En móvil y navegador hay eventos de suspensión; Ebitengine pausa el audio y el
  bucle. No asumas que el tiempo real transcurrido coincide con los ticks.

Para una pausa real, detén la simulación, no el bucle: sigue llamando a `Draw`
para mostrar el menú y sigue leyendo input para reanudar.

## 7. Depuración

Lo mínimo:

```go
import "github.com/hajimehoshi/ebiten/v2/ebitenutil"

func (g *Game) Draw(screen *ebiten.Image) {
	// ...
	ebitenutil.DebugPrint(screen, fmt.Sprintf(
		"TPS: %.1f  FPS: %.1f\nentidades: %d",
		ebiten.ActualTPS(), ebiten.ActualFPS(), len(g.entities),
	))
}
```

Para cajas de colisión y estados, dibuja rectángulos con `vector`:

```go
import "github.com/hajimehoshi/ebiten/v2/vector"

vector.StrokeRect(screen, x, y, w, h, 1, color.RGBA{0xff, 0, 0, 0xff}, false)
```

Consejos:

- Alterna el modo debug con una tecla (F1) y no lo dejes siempre activo.
- Muestra estado, tick de ataque, vida, invulnerabilidad y número de entidades.
- Para inspeccionar el render real, ejecuta con `-tags=ebitenginedebug`; el motor
  imprime las comandos de dibujo internos.
- Si el juego va lento, mide antes de optimizar: `ActualTPS` y `ActualFPS` suelen
  delatar si el problema está en `Update` o en `Draw`.

## 8. Errores frecuentes

| Síntoma | Causa probable |
|---|---|
| El personaje se mueve distinto en otro monitor | Lógica atada a frames de `Draw` o a tiempo real |
| La animación va a saltos | Usar `time.Now()` o acumular floats sin redondear |
| El input «se pierde» | Leer `JustPressed` en `Draw` o consumirlo dos veces |
| Todo va lento tras un rato | Crear imágenes, slices o fuentes en cada frame |
| El pixel art se ve borroso | Escala no entera o `FilterLinear` |
| El juego se estira raro al redimensionar | `Layout` mal definido o falta letterboxing |
| Los sprites tiemblan | Posiciones/cámara fraccionarias dibujadas sin redondear |
| El juego sigue en segundo plano | No comprobar `ebiten.IsFocused()` |

## 9. Ejercicio del capítulo

Sin mirar los ejemplos:

1. Crea una ventana de 480×270 lógica y 960×540 real.
2. Dibuja un rectángulo de 16×32 que represente al jugador.
3. Muévelo con WASD y flechas, con velocidad constante y límites de pantalla.
4. Añade aceleración y fricción: al soltar la tecla debe deslizarse un poco.
5. Dibuja un segundo rectángulo que sea un obstáculo y muestra su caja.
6. Muestra TPS, FPS y la posición del jugador con `DebugPrint`.
7. Pausa el movimiento cuando `!ebiten.IsFocused()`.

Criterio de aceptación: el movimiento se siente igual si cambias el tamaño de la
ventana, y la posición mostrada coincide con la caja dibujada.

## 10. Referencias

- Interfaz `Game`, bucle, TPS: `run.go`, `gameforui.go`.
- Matrices: `geom.go`.
- Imágenes, subimágenes y opciones: `image.go`.
- Entrada: `input.go`, `inpututil/`.
- Ventana: `window.go`, `run.go`.
- Ejemplos: `examples/platformer`, `examples/camera`, `examples/animation`.
- Documentación: <https://pkg.go.dev/github.com/hajimehoshi/ebiten/v2>
