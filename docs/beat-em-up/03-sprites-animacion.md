# 03 — Sprites, animación y orden de dibujo

En un beat 'em up, la animación no es decoración: comunica el alcance, el momento
del golpe y el estado del personaje. Un sprite mal anclado o dibujado en el orden
equivocado arruina la lectura del combate aunque la lógica sea correcta.

## 1. Del sprite sheet al frame

Un *sprite sheet* es una imagen con varios frames en una cuadrícula. Lo habitual
es una fila por animación:

```text
idle:    [0][1][2][3]
walk:    [0][1][2][3][4][5]
attack:  [0][1][2][3][4]
hurt:    [0][1]
```

```go
type Frame struct {
	Rect     image.Rectangle
	Duration int // ticks; 0 se trata como 1
}

type Clip struct {
	Name   string
	Frames []Frame
	Loop   bool
}
```

Helper para construir frames de una cuadrícula:

```go
func gridClip(name string, row, frameW, frameH, count, duration int, loop bool) *Clip {
	c := &Clip{Name: name, Loop: loop}
	for i := 0; i < count; i++ {
		x := i * frameW
		y := row * frameH
		c.Frames = append(c.Frames, Frame{
			Rect:     image.Rect(x, y, x+frameW, y+frameH),
			Duration: duration,
		})
	}
	return c
}
```

Y para obtener la imagen:

```go
func (c *Clip) Image(sheet *ebiten.Image, i int) *ebiten.Image {
	if len(c.Frames) == 0 {
		return nil
	}
	if i < 0 {
		i = 0
	}
	if i >= len(c.Frames) {
		i = len(c.Frames) - 1
	}
	return sheet.SubImage(c.Frames[i].Rect).(*ebiten.Image)
}
```

`SubImage` no copia píxeles: comparte la textura. Es barato y permite tener todas
las animaciones en un solo PNG. Ojo: la subimagen se dibuja como una imagen
pequeña con origen en `(0, 0)`; no uses `Rect.Min` para colocarla.

### Metadatos vs cuadrícula fija

La cuadrícula fija es suficiente si todos los frames tienen el mismo tamaño. Si
tus sprites tienen recortes distintos, guarda un JSON con el rectángulo de cada
frame (formato tipo Aseprite/TexturePacker) y cárgalo. No inventes offsets a ojo:
un frame mal recortado produce un «salto» visible al cambiar de animación.

## 2. El animador

```go
type Animator struct {
	Clips   map[string]*Clip
	Current *Clip
	Frame   int
	Ticks   int // ticks dentro del frame actual
	Done    bool
}

func (a *Animator) Play(name string, restart bool) {
	c := a.Clips[name]
	if c == nil {
		return
	}
	if a.Current == c && !restart {
		return
	}
	a.Current = c
	a.Frame = 0
	a.Ticks = 0
	a.Done = false
}

func (a *Animator) Update() {
	if a.Current == nil || a.Done {
		return
	}
	a.Ticks++
	d := a.Current.Frames[a.Frame].Duration
	if d <= 0 {
		d = 1
	}
	if a.Ticks < d {
		return
	}
	a.Ticks = 0
	a.Frame++
	if a.Frame < len(a.Current.Frames) {
		return
	}
	if a.Current.Loop {
		a.Frame = 0
		return
	}
	a.Frame = len(a.Current.Frames) - 1
	a.Done = true
}
```

Uso:

```go
a.Play("walk", false)
a.Update()
img := a.Current.Image(sheet, a.Frame)
```

### ¿Quién manda: la animación o el estado de combate?

Hay dos escuelas:

1. **La animación manda.** Un evento en el frame 3 activa la hitbox. Es intuitivo
   para artistas, pero mezcla presentación y simulación, y complica los tests.
2. **El estado de combate manda.** El ataque tiene `startup/active/recovery` en
   ticks; la animación es una consecuencia visual. Es más fácil de testear,
   balancear y reproducir.

Para un beat 'em up, recomiendo la segunda. El `Move` define los ticks; el
`Animator` elige el frame visual aproximado:

```go
func (e *Entity) syncAnimation() {
	switch e.State {
	case StateIdle:
		e.Anim.Play("idle", false)
	case StateWalk:
		e.Anim.Play("walk", false)
	case StateAttack:
		e.Anim.Play(e.Move.Anim, true)
		// Si la animación tiene menos frames que el ataque, se queda en el último.
	case StateHurt:
		e.Anim.Play("hurt", true)
	case StateDown:
		e.Anim.Play("down", true)
	case StateDead:
		e.Anim.Play("dead", true)
	}
}
```

Si necesitas que un efecto (polvo, sonido, chispa) ocurra en un frame concreto,
dispara el evento desde el contador de combate, no desde `Draw`.

## 3. Anclaje: los pies son el origen

Para ordenar por profundidad y para que la sombra coincida, el punto de referencia
de un personaje debe ser **el centro de sus pies** en el suelo. No el centro del
sprite, no la esquina superior izquierda.

```go
func drawEntity(dst *ebiten.Image, sheet *ebiten.Image, e *Entity, cam Camera) {
	if e.Anim.Current == nil {
		return
	}
	img := e.Anim.Current.Image(sheet, e.Anim.Frame)
	if img == nil {
		return
	}
	w := float64(img.Bounds().Dx())
	h := float64(img.Bounds().Dy())

	sx, sy := cam.WorldToScreen(e.Pos.X, e.Pos.Y, e.Pos.Z)
	sx = math.Round(sx)
	sy = math.Round(sy)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-w/2, -h) // el origen pasa a ser el centro de los pies
	if e.Facing < 0 {
		op.GeoM.Scale(-1, 1) // espejo alrededor del eje de los pies
	}
	op.GeoM.Translate(sx, sy)
	dst.DrawImage(img, op)
}
```

Orden de las operaciones:

1. `Translate(-w/2, -h)`: mueve la imagen para que los pies queden en `(0, 0)`.
2. `Scale(-1, 1)`: espeja sin mover el punto de los pies.
3. `Translate(sx, sy)`: coloca los pies en la pantalla.

Si cambias el orden, el personaje se desplaza medio ancho al girar. Ese es el bug
clásico de «el sprite salta al mirar a la izquierda».

### Escala por profundidad (opcional)

Para dar perspectiva, los personajes más cercanos pueden ser un poco más grandes:

```go
scale := 1.0 + 0.15*(e.Pos.Y-minY)/(maxY-minY)
op.GeoM.Translate(-w/2, -h)
if e.Facing < 0 {
	op.GeoM.Scale(-1, 1)
}
op.GeoM.Scale(scale, scale)
op.GeoM.Translate(sx, sy)
```

Con pixel art, la escala fraccionaria produce frames de distinto tamaño y puede
verse sucio. Alternativas: no escalar, escalar solo entre dos valores (1× y 1.1×)
o renderizar el mundo a una resolución mayor. Decide y sé consistente: las cajas
de colisión **no** deben escalar con el dibujo.

## 4. Orden de dibujo: pintor por profundidad

El suelo es un plano: un personaje con `Y` mayor está más cerca de la cámara y
debe tapar al que tiene `Y` menor. Ordena por los pies:

```go
func sortForDrawing(entities []*Entity) {
	slices.SortStableFunc(entities, func(a, b *Entity) int {
		if a.Pos.Y != b.Pos.Y {
			return cmp.Compare(a.Pos.Y, b.Pos.Y)
		}
		// Desempate estable y determinista.
		return cmp.Compare(a.ID, b.ID)
	})
}
```

Detalles:

- **Ordena una copia o una lista de índices**, no la lista que actualizas, para
  no alterar el orden de simulación.
- Si dos entidades comparten `Y`, el desempate por `ID` evita parpadeos.
- Las sombras se dibujan antes que todos los personajes.
- Los efectos aéreos (saltos, proyectiles) pueden necesitar una capa propia.
- El HUD se dibuja después y sin cámara.

### Capas

```go
const (
	LayerGround = iota // sombras, decorado bajo
	LayerWorld         // personajes ordenados por Y
	LayerAir           // proyectiles, partículas por encima
	LayerUI
)
```

No hace falta un sistema de capas complejo; con listas separadas y un orden fijo
se entiende mejor.

## 5. Sombra, destello y parpadeo

### Sombra

Una sombra anclada a los pies ayuda a leer la altura (`Z`) y la profundidad (`Y`).
Lo más simple es un sprite de óvalo semitransparente dibujado en
`(Pos.X, Pos.Y)` con la cámara aplicada. Si no tienes el asset, genera una imagen
una vez al cargar y reutilízala:

```go
func makeShadow(w, h int) *ebiten.Image {
	img := ebiten.NewImage(w, h)
	// Dibuja un óvalo con vector.Path o copia un PNG.
	return img
}
```

No dibujes la sombra con la misma escala que el personaje si el personaje está
saltando: la sombra se queda en el suelo.

### Destello de impacto

`ColorScale` multiplica los canales; no convierte mágicamente el sprite en blanco:

```go
op.ColorScale.Set(1.5, 1.5, 1.5, 1)
```

Para un destello blanco puro, pre-renderiza una silueta blanca del frame (usando
el canal alfa) o usa un shader. Para el primer prototipo, un tinte claro y un
par de ticks es suficiente.

### Parpadeo de invulnerabilidad

Hazlo determinista, basado en ticks, no en `time.Now()`:

```go
if e.Invuln > 0 && (e.Invuln/3)%2 == 0 {
	return // no dibujar este tick
}
```

Así el parpadeo se ve igual en todas las máquinas y en las repeticiones.

## 6. Animación y estados: una tabla clara

Define qué animación corresponde a cada estado y cuándo se reinicia:

| Estado | Animación | Loop | Reinicio |
|---|---|---|---|
| `Idle` | `idle` | sí | al entrar |
| `Walk` | `walk` | sí | al entrar |
| `Attack` | `move.Anim` | no | al iniciar el golpe |
| `Block` | `block` | sí | al entrar |
| `Hurt` | `hurt` | no | al recibir daño |
| `Down` | `down` | no | al caer |
| `GetUp` | `getup` | no | al levantarse |
| `Dead` | `dead` | no | al morir |

Regla: `Play(name, restart=true)` al **entrar** en un estado; `Play(name, false)`
cada tick para que los cambios externos (por ejemplo, un ataque nuevo) no se
ignoren. Si `Play` con `restart=false` no hace nada cuando ya está reproduciendo,
puedes llamarlo sin miedo cada tick.

### Cuándo termina una animación

`Animator.Done` es visual. La transición de `Attack` a `Idle` la decide el frame
data del ataque, no el final del PNG. Si dependes del sprite, un artista que añade
un frame cambia el balance del juego sin querer.

## 7. Pipeline de assets

Recomendaciones prácticas:

- Un PNG por personaje con todas sus animaciones; usa filas y nombres claros.
- Deja 1–2 píxeles de padding entre frames si usas filtro lineal o escalado.
- Nombra los frames con la misma convención que el código: `idle`, `walk`,
  `attack_1`, `attack_2`, `hurt`, `down`, `getup`.
- Mantén el mismo tamaño de lienzo por personaje y la misma línea de pies. Si un
  frame es más alto, el anclaje de pies seguirá funcionando, pero la cabeza puede
  cambiar de altura; es lo esperado.
- Versiona los assets junto al código. Un cambio de sprite sin actualizar el
  frame data produce golpes que «no conectan».
- Para pixel art, desactiva la compresión con pérdida y evita reescalar el PNG.

## 8. Errores frecuentes

| Síntoma | Causa probable |
|---|---|
| El personaje salta al girar | Espejo sin centrar en los pies |
| El sprite se dibuja en el sitio equivocado | Usar `Bounds().Min` de la subimagen |
| La animación se acelera al cambiar TPS | Duraciones en segundos mezcladas con ticks |
| El ataque termina antes que la animación | Transición basada en frames visuales |
| Un enemigo tapa a otro al azar | Orden no determinista o sin ordenar por `Y` |
| El personaje tiembla | Posiciones fraccionarias sin redondear |
| El destello no se ve | `ColorScale` no hace blanco puro |
| La sombra flota | Sombra dibujada con `Z` en vez de `Y` |

## 9. Ejercicio del capítulo

1. Prepara un sprite sheet de 4 animaciones: `idle`, `walk`, `attack`, `hurt`.
2. Implementa `Frame`, `Clip` y `Animator` con duraciones en ticks.
3. Dibuja al personaje anclado por los pies, con espejo correcto.
4. Añade una sombra en el suelo y comprueba que no se mueve al saltar.
5. Dibuja tres personajes con distinto `Y` y verifica el orden.
6. Añade un destello de 2 ticks al recibir daño y parpadeo de invulnerabilidad.
7. Fuerza un error: cambia el orden de `Scale` y `Translate` y observa el salto;
   vuelve a corregirlo.

Criterio de aceptación: al girar, los pies no se mueven; al cambiar de `Y`, el
orden visual cambia; el ataque se ve sincronizado con el frame data.

## 10. Referencias

- `examples/animation` para `SubImage` y frames.
- `image.go`: `SubImage`, `DrawImage`, `DrawImageOptions`, `ColorScale`.
- `geom.go`: orden de transformaciones.
- `text/v2` no interviene aquí, pero comparte el concepto de recursos cacheados.
