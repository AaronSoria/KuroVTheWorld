# 08 — HUD, texto y menús

La interfaz de un beat 'em up debe responder a tres preguntas en menos de un
segundo: cuánta vida me queda, cuántos enemigos hay y qué puedo hacer ahora. El
resto es estilo. Este capítulo cubre HUD, texto, menús y pausa.

## 1. Dos espacios, dos capas

- El **mundo** se dibuja con la cámara aplicada.
- La **UI** se dibuja después, en píxeles de pantalla, sin cámara.

```go
func (s *PlayScene) Draw(screen *ebiten.Image) {
	screen.Fill(backgroundColor)
	s.world.Draw(screen) // mundo + cámara
	s.world.DrawPopups(screen)
	s.hud.Draw(screen)   // vida, combo, tiempo
	if s.paused {
		s.pauseMenu.Draw(screen)
	}
}
```

Nunca apliques la cámara al HUD. Si quieres que un elemento del mundo (una barra
sobre un enemigo) siga a la cámara, conviértelo explícitamente:

```go
sx, sy := cam.WorldToScreen(e.Pos.X, e.Pos.Y, e.Pos.Z)
```

## 2. Texto con `text/v2`

`text/v2` es la API recomendada para texto nuevo. La v1 (`text.Draw` con
`font.Face`) sigue existiendo, pero la v2 maneja mejor fuentes variables, color
y alineación.

### Cargar una fuente una vez

```go
//go:embed assets/fonts/ui.ttf
var uiFontData []byte

type UI struct {
	Face *text.GoTextFace
}

func NewUI() (*UI, error) {
	src, err := text.NewGoTextFaceSource(bytes.NewReader(uiFontData))
	if err != nil {
		return nil, err
	}
	return &UI{
		Face: &text.GoTextFace{Source: src, Size: 16},
	}, nil
}
```

`NewGoTextFaceSource` parsea la fuente; hazlo una vez. Crear una `GoTextFace` es
barato, pero crear la fuente cada frame no lo es.

### Dibujar

```go
op := &text.DrawOptions{}
op.GeoM.Translate(8, 8)
op.ColorScale.ScaleWithColor(color.White)
text.Draw(screen, "VIDA", ui.Face, op)
```

En `text/v2`, la posición `(0, 0)` es la **esquina superior izquierda** del área
de texto (a diferencia de la v1, que usaba la línea base). Si vienes de la v1,
este es el cambio que más te va a confundir.

### Alineación

```go
op := &text.DrawOptions{}
op.PrimaryAlign = text.AlignCenter
op.SecondaryAlign = text.AlignCenter
op.GeoM.Translate(ScreenWidth/2, ScreenHeight/2)
text.Draw(screen, "GAME OVER", bigFace, op)
```

- `PrimaryAlign`: horizontal en una fuente horizontal.
- `SecondaryAlign`: vertical.
- El `GeoM` sitúa el punto de referencia; la alineación decide cómo se coloca el
  texto respecto a ese punto.

### Medir

```go
w, h := text.Measure("COMBO x12", ui.Face, 0)
```

`Measure` es útil para centrar, para fondos de botones y para comprobar que un
texto cabe. El tercer argumento es el espaciado entre líneas: para una sola línea
puedes pasar 0; para varias, pasa un valor explícito (por ejemplo, el tamaño de
la fuente).

### Precargar glifos

Si el HUD cambia mucho, puedes rasterizar los glifos usados al arrancar:

```go
text.CacheGlyphs(ui.Face, "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZÁÉÍÓÚÑ:¡!?")
```

No es obligatorio, pero evita un pico la primera vez que aparece un carácter.

### Fuentes pixel art

- Usa un tamaño entero y `op.Filter = ebiten.FilterNearest` (heredado de
  `DrawImageOptions`).
- Comprueba que la fuente tiene los glifos que necesitas (tildes, ñ, ¿, ¡).
- Si la fuente es de mapa de bits, quizá prefieras `ebitenutil.DebugPrint` para
  el debug y una fuente TTF pixelada para la UI.
- No uses `DebugPrint` en producción: su fuente y su estilo son de depuración.

## 3. HUD

### Barra de vida

```go
func drawBar(dst *ebiten.Image, x, y, w, h float32, ratio float64, fg, bg color.Color) {
	ratio = math.Max(0, math.Min(1, ratio))
	vector.DrawFilledRect(dst, x, y, w, h, bg, false)
	vector.DrawFilledRect(dst, x, y, w*float32(ratio), h, fg, false)
	vector.StrokeRect(dst, x, y, w, h, 1, color.RGBA{0, 0, 0, 0xff}, false)
}
```

Ideas de presentación:

- Vida del jugador arriba a la izquierda, con retrato y nombre.
- Vida del jefe arriba o abajo, más grande, cuando aparece.
- Barras de enemigos comunes: solo si aportan información; demasiadas barras
  ensucian la pantalla.
- Usa un color de «daño reciente» que se vacíe con retardo para que el jugador
  vea cuánto acaba de perder.
- No confíes solo en el color: añade forma, texto o iconos.

### Combo

```go
if w.Player.ComboCount > 1 {
	msg := fmt.Sprintf("%d HITS", w.Player.ComboCount)
	w, h := text.Measure(msg, ui.Face, 0)
	op := &text.DrawOptions{}
	op.GeoM.Translate(float64(ScreenWidth)-w-16, 16)
	op.ColorScale.ScaleWithColor(color.RGBA{0xff, 0xd0, 0x40, 0xff})
	text.Draw(screen, msg, ui.Face, op)
}
```

Un pequeño «pop» de escala al aumentar el contador da energía. Hazlo con un
temporizador en ticks y `GeoM.Scale`, no con un tween infinito.

### Tiempo y puntuación

- El tiempo límite es opcional; si lo pones, que sea generoso.
- La puntuación puede premiar combos, no solo kills.
- Muestra el objetivo de la oleada («DERROTA A TODOS») cuando sea necesario.

### Números de daño

```go
type Popup struct {
	Text  string
	Pos   Vec3
	Ticks int
}

func (p *Popup) Draw(dst *ebiten.Image, cam Camera, face *text.GoTextFace) {
	sx, sy := cam.WorldToScreen(p.Pos.X, p.Pos.Y, p.Pos.Z)
	op := &text.DrawOptions{}
	op.GeoM.Translate(sx, sy-float64(p.Ticks))
	op.PrimaryAlign = text.AlignCenter
	op.ColorScale.ScaleAlpha(float32(p.Ticks) / 30)
	text.Draw(dst, p.Text, face, op)
}
```

Los números de daño son opcionales; en un beat 'em up clásico no siempre se
muestran. Si los usas, que sean legibles y no tapen la acción.

## 4. Menús

### Menú vertical simple

```go
type Menu struct {
	Items   []string
	Index   int
	Repeat  int
	Confirm bool
	Cancel  bool
}

func (m *Menu) Update(in InputState) {
	dir := 0
	if in.MenuDown {
		dir = 1
	} else if in.MenuUp {
		dir = -1
	}

	if dir != 0 {
		if m.Repeat == 0 {
			m.Index = (m.Index + dir + len(m.Items)) % len(m.Items)
			m.Repeat = 12 // retardo inicial
		} else {
			m.Repeat--
			if m.Repeat == 0 {
				m.Repeat = 6 // repetición
			}
		}
	} else {
		m.Repeat = 0
	}

	m.Confirm = in.ConfirmPressed
	m.Cancel = in.CancelPressed
}
```

Detalles de UX:

- La navegación debe repetirse al mantener, con retardo inicial; si no, cuesta
  moverse por listas largas.
- Confirmar y cancelar deben ser botones distintos y consistentes.
- El botón «atrás» del mando debe funcionar como cancelar.
- No mezcles input de menú con input de combate en el mismo buffer sin querer.
- Si el menú se abre con pausa, el mundo no debe actualizarse, pero el menú sí.

### Pausa

```go
func (s *PlayScene) Update() (Scene, error) {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		s.paused = !s.paused
	}
	if s.paused {
		s.pauseMenu.Update(s.input.MenuState())
		if s.pauseMenu.Confirm {
			switch s.pauseMenu.Index {
			case 0: // continuar
				s.paused = false
			case 1: // opciones
				return NewOptionsScene(s), nil
			case 2: // salir al título
				return NewTitleScene(), nil
			}
		}
		return nil, nil
	}
	s.world.Update(s.input.State())
	return nil, nil
}
```

Para cerrar el juego desde el menú, devuelve `nil, ebiten.Termination`; Ebitengine
detiene el bucle sin tratarlo como un error.

Dibuja el mundo congelado debajo y un velo. Crea el velo una sola vez (por
ejemplo, en `NewPlayScene` o de forma perezosa) y reutilízalo:

```go
if s.overlay == nil {
	s.overlay = ebiten.NewImage(ScreenWidth, ScreenHeight)
	s.overlay.Fill(color.RGBA{0, 0, 0, 0x99})
}
screen.DrawImage(s.overlay, nil)
```

### Opciones

Controles típicos:

- Volumen master / música / SFX.
- Dificultad.
- Sacudida de cámara y destellos.
- Remapeo de teclas.
- Idioma.

Un slider se implementa con izquierda/derecha y un paso:

```go
if in.MenuLeft {
	v = math.Max(0, v-0.1)
}
if in.MenuRight {
	v = math.Min(1, v+0.1)
}
```

Muestra el valor numérico o una barra; no obligues a adivinar.

### Transiciones

Un fundido sencillo:

```go
type Fade struct {
	Ticks int
	Total int
	In    bool
}

func (f *Fade) Alpha() float32 {
	if f.Total == 0 {
		return 0
	}
	t := float32(f.Ticks) / float32(f.Total)
	if !f.In {
		t = 1 - t
	}
	return t
}
```

Dibuja un rectángulo negro con alfa encima de todo. No bloquees la carga de
assets con una transición; carga antes o muestra una pantalla de carga.

## 5. Accesibilidad y legibilidad

- **Contraste**: el texto debe leerse sobre fondos claros y oscuros. Añade una
  sombra o un panel semitransparente.
- **Tamaño**: no uses fuentes de 8 px para información importante.
- **Color**: no uses solo rojo/verde para vida o estados; añade iconos o texto.
- **Sacudida y destellos**: ofrece opciones para reducirlos. Los destellos fuertes
  pueden afectar a personas fotosensibles.
- **Remapeo**: permite cambiar teclas y muestra el mapeo actual.
- **Pausa**: debe funcionar siempre, incluso en una cinemática (o saltarla).
- **Idioma**: centraliza las cadenas; no las repartas por el código.

```go
type Strings struct {
	NewGame  string
	Continue string
	Options  string
	Quit     string
	GameOver string
}
```

## 6. Errores frecuentes

| Síntoma | Causa probable |
|---|---|
| El texto sale desplazado | Se usó la semántica de línea base de la v1 |
| El HUD se mueve con la cámara | Se dibujó dentro del mundo |
| El menú se salta dos opciones | `JustPressed` y repetición a la vez |
| El texto no tiene tildes | La fuente no cubre los glifos |
| El texto se ve borroso | Escala fraccionaria o filtro lineal |
| El menú no responde al mando | Falta mapear el botón estándar |
| La pausa sigue actualizando el mundo | Se llama a `world.Update` antes del `if` |
| El overlay se crea cada frame | Falta cachear la imagen |

## 7. Ejercicio del capítulo

1. Carga una fuente TTF con `text/v2` y dibuja «MI BEAT 'EM UP» centrado.
2. Implementa una barra de vida con daño retardado.
3. Añade un contador de combo que aparezca y desaparezca con temporizador.
4. Crea un menú de pausa con Continuar, Opciones y Salir.
5. Implementa navegación con teclado y mando, con retardo y repetición.
6. Añade un slider de volumen que actualice el audio en caliente.
7. Dibuja un número de daño que suba y se desvanezca.
8. Comprueba que todo el texto se lee a 480×270 y a 1920×1080.

Criterio de aceptación: puedes pausar, cambiar el volumen, reanudar y salir sin
que el mundo avance mientras el menú está abierto.

## 8. Referencias

- `text/v2`: `NewGoTextFaceSource`, `GoTextFace`, `Draw`, `Measure`,
  `CacheGlyphs`, `DrawOptions`.
- `vector`: `DrawFilledRect`, `StrokeRect`, `DrawFilledCircle`.
- `examples/text`, `examples/ui`, `examples/textinput`.
- `ebitenutil.DebugPrint` solo para debug.
