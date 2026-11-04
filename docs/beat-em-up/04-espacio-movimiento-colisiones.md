# 04 — Espacio, movimiento y colisiones

El beat 'em up es un juego 2D que simula un plano con profundidad. Si confundes
los ejes, todo lo demás se complica: los golpes no conectan, los personajes se
tapan mal y las sombras flotan. Este capítulo fija el modelo espacial y las
colisiones mínimas.

## 1. El modelo 2.5D

Usa tres coordenadas:

```text
X: lateral. + derecha.
Y: profundidad del suelo. + hacia la cámara (abajo en pantalla).
Z: altura sobre el suelo. + arriba.
```

La pantalla proyecta:

```text
screenX = worldX - cameraX
screenY = worldY - worldZ - cameraY
```

```go
type Vec3 struct {
	X float64
	Y float64
	Z float64
}
```

Consecuencias:

- Caminar por la acera cambia `X`; acercarse o alejarse cambia `Y`.
- Saltar cambia `Z`, no `Y`. La sombra se queda en `(X, Y)`.
- La profundidad visual de un personaje es su `Y` de los pies, no su `Z`.
- Un proyectil puede volar (`Z > 0`) y seguir teniendo una `Y` de suelo.

La zona jugable es una banda:

```go
type Bounds struct {
	MinX, MaxX float64
	MinY, MaxY float64
}
```

`MinY/MaxY` definen la profundidad por la que se puede caminar. Si el escenario
tiene una acera y una calzada, puedes modelarlas como dos bandas o como una sola
con decorado. Para el primer nivel, una banda es suficiente.

## 2. Cuerpo, hurtbox y hitbox

No uses la imagen para colisionar. Define cajas explícitas:

```go
type AABB struct {
	MinX, MinY float64
	MaxX, MaxY float64
}

func (a AABB) Overlaps(b AABB) bool {
	return a.MinX < b.MaxX && b.MinX < a.MaxX &&
		a.MinY < b.MaxY && b.MinY < a.MaxY
}

func (a AABB) Width() float64  { return a.MaxX - a.MinX }
func (a AABB) Height() float64 { return a.MaxY - a.MinY }
```

### Cuerpo físico

El cuerpo ocupa un rectángulo en el plano `X/Y`, centrado en los pies:

```go
type Box struct {
	Width  float64 // ancho lateral
	Depth  float64 // profundidad
	Height float64
}

func (e *Entity) BodyBox() AABB {
	hw := e.Body.Width / 2
	hd := e.Body.Depth / 2
	return AABB{
		MinX: e.Pos.X - hw, MaxX: e.Pos.X + hw,
		MinY: e.Pos.Y - hd, MaxY: e.Pos.Y + hd,
	}
}
```

`Depth` suele ser pequeño (16–24 px) para que los personajes puedan alinearse.
Si es igual al ancho, el juego se siente «cuadrado» y cuesta colocarse delante de
un enemigo.

### Hurtbox

La hurtbox es la zona donde te pueden golpear. Normalmente coincide con el cuerpo,
a veces un poco más generosa:

```go
func (e *Entity) Hurtbox() AABB {
	b := e.BodyBox()
	const margin = 2.0
	b.MinX -= margin
	b.MaxX += margin
	b.MinY -= margin / 2
	b.MaxY += margin / 2
	return b
}
```

No la hagas gigante «para que sea más fácil acertar»: el jugador lo nota y el
combate pierde precisión. Ajusta con debug visible.

### Hitbox

Una hitbox pertenece a un ataque y se define relativa a los pies:

```go
type Hitbox struct {
	OffsetX float64 // centro lateral respecto a los pies
	OffsetY float64 // centro de profundidad respecto a los pies
	Width   float64
	Depth   float64
	ZMin    float64 // altura mínima, relativa al suelo
	ZMax    float64 // altura máxima
}
```

```go
func (h Hitbox) World(e *Entity) AABB {
	cx := e.Pos.X + h.OffsetX*float64(e.Facing) // el offset se espeja
	cy := e.Pos.Y + h.OffsetY
	return AABB{
		MinX: cx - h.Width/2, MaxX: cx + h.Width/2,
		MinY: cy - h.Depth/2, MaxY: cy + h.Depth/2,
	}
}
```

Si el personaje salta, la hitbox sube con él. Para eso, la comprobación en `Z`
suma `e.Pos.Z`:

```go
func overlapsZ(a *Entity, h Hitbox, b *Entity) bool {
	aMin := h.ZMin + a.Pos.Z
	aMax := h.ZMax + a.Pos.Z
	bMin := b.Pos.Z
	bMax := b.Pos.Z + b.Body.Height
	return aMin < bMax && bMin < aMax
}
```

Sin la comprobación en `Z`, un ataque aéreo golpearía a alguien que está en el
suelo detrás de él. Para el primer prototipo puedes ignorar `Z` en los golpes
básicos, pero los saltos y los proyectiles la necesitan.

## 3. Movimiento

### Velocidad en píxeles por tick

Elige una unidad y sé consistente. Recomiendo píxeles por tick:

```go
const (
	walkSpeed  = 2.5  // 150 px/s a 60 TPS
	depthSpeed = 2.0  // 120 px/s
)
```

Movimiento diagonal normalizado:

```go
func moveInput(in InputState) (dx, dy float64) {
	dx = in.MoveX
	dy = in.MoveY
	l := math.Hypot(dx, dy)
	if l > 1 {
		dx /= l
		dy /= l
	}
	return dx, dy
}
```

Sin normalizar, moverse en diagonal es un 41 % más rápido. Es un bug clásico y
muy visible en un beat 'em up.

### Integración y límites

```go
func (w *World) moveEntity(e *Entity, dx, dy float64) {
	e.Pos.X += dx * e.WalkSpeedX
	e.Pos.Y += dy * e.WalkSpeedY
	w.clampToBounds(e)
}

func (w *World) clampToBounds(e *Entity) {
	hw := e.Body.Width / 2
	hd := e.Body.Depth / 2
	e.Pos.X = math.Max(w.Bounds.MinX+hw, math.Min(w.Bounds.MaxX-hw, e.Pos.X))
	e.Pos.Y = math.Max(w.Bounds.MinY+hd, math.Min(w.Bounds.MaxY-hd, e.Pos.Y))
}
```

### Aceleración y fricción

Para un beat 'em up clásico, el movimiento suele ser casi instantáneo. Si quieres
inercia:

```go
const (
	accel = 0.8
	friction = 0.6
)

func approach(v, target, step float64) float64 {
	if v < target {
		return math.Min(v+step, target)
	}
	return math.Max(v-step, target)
}

e.Vel.X = approach(e.Vel.X, targetX, accel)
e.Vel.X *= friction
```

La fricción debe aplicarse con cuidado: si multiplicas cada tick, la velocidad
nunca llega a cero. Corta a cero por debajo de un umbral.

### Saltos

```go
const gravity = 0.35 // px/tick²

func (w *World) updateJump(e *Entity) {
	if e.Pos.Z <= 0 && e.Vel.Z <= 0 {
		e.Pos.Z = 0
		e.Vel.Z = 0
		return
	}
	e.Vel.Z -= gravity
	e.Pos.Z += e.Vel.Z
	if e.Pos.Z <= 0 {
		e.Pos.Z = 0
		e.Vel.Z = 0
		// aterrizaje: polvo, sonido, transición de estado
	}
}
```

Con `Z` positivo hacia arriba, la gravedad resta. El movimiento lateral en el aire
puede ser libre o limitado; decide y documenta la regla. Un beat 'em up suele
permitir cierto control aéreo, pero no tanto como un plataformas.

## 4. Colisiones con el escenario

Modela paredes y obstáculos como AABB en el plano del suelo:

```go
type Wall struct {
	Box AABB
}
```

Resuelve **eje por eje** para que el personaje se deslice en lugar de quedarse
pegado:

```go
func (w *World) moveWithWalls(e *Entity, dx, dy float64) {
	// Eje X.
	e.Pos.X += dx
	for _, wall := range w.Walls {
		if !e.BodyBox().Overlaps(wall.Box) {
			continue
		}
		if dx > 0 {
			e.Pos.X = wall.Box.MinX - e.Body.Width/2
		} else if dx < 0 {
			e.Pos.X = wall.Box.MaxX + e.Body.Width/2
		}
	}

	// Eje Y.
	e.Pos.Y += dy
	for _, wall := range w.Walls {
		if !e.BodyBox().Overlaps(wall.Box) {
			continue
		}
		if dy > 0 {
			e.Pos.Y = wall.Box.MinY - e.Body.Depth/2
		} else if dy < 0 {
			e.Pos.Y = wall.Box.MaxY + e.Body.Depth/2
		}
	}
}
```

Detalles:

- Con varias paredes, el bucle puede resolver una y entrar en otra; repite si tu
  escenario tiene esquinas complicadas.
- Usa un `epsilon` (0.01) al colocar para evitar que `Overlaps` siga siendo cierto
  por error de coma flotante.
- No resuelvas X e Y a la vez: perderías el deslizamiento.
- Si la velocidad es mayor que el grosor de la pared, el personaje la atraviesa
  (*tunneling*). Limita la velocidad o subdivide el movimiento en pasos.

## 5. Colisión entre personajes

Que dos cuerpos ocupen el mismo espacio se ve mal y rompe la lectura. Una
separación suave (empuje) es suficiente:

```go
func (w *World) separateBodies() {
	for i := 0; i < len(w.Entities); i++ {
		a := w.Entities[i]
		if !a.Alive {
			continue
		}
		for j := i + 1; j < len(w.Entities); j++ {
			b := w.Entities[j]
			if !b.Alive || a.Team == b.Team && !a.SolidWithAllies {
				continue
			}
			aBox, bBox := a.BodyBox(), b.BodyBox()
			if !aBox.Overlaps(bBox) {
				continue
			}
			// Si uno está muy alto, no lo empujes contra el del suelo.
			if a.Pos.Z > a.Body.Height || b.Pos.Z > b.Body.Height {
				continue
			}

			overlapX := math.Min(aBox.MaxX, bBox.MaxX) - math.Max(aBox.MinX, bBox.MinX)
			overlapY := math.Min(aBox.MaxY, bBox.MaxY) - math.Max(aBox.MinY, bBox.MinY)
			if overlapX <= 0 || overlapY <= 0 {
				continue
			}

			if overlapX < overlapY {
				push := overlapX / 2
				if a.Pos.X < b.Pos.X {
					a.Pos.X -= push
					b.Pos.X += push
				} else {
					a.Pos.X += push
					b.Pos.X -= push
				}
			} else {
				push := overlapY / 2
				if a.Pos.Y < b.Pos.Y {
					a.Pos.Y -= push
					b.Pos.Y += push
				} else {
					a.Pos.Y += push
					b.Pos.Y -= push
				}
			}
		}
	}
}
```

Consideraciones:

- Aplica la separación **después** del movimiento intencional y **antes** de
  detectar golpes, para que las cajas reflejen la posición final.
- Vuelve a limitar a los bordes del escenario después de empujar. Si no, puedes
  sacar a alguien de la zona jugable.
- Si los centros coinciden exactamente, elige el eje X y un signo determinista
  por `ID`; si no, el resultado depende del orden.
- Decide si los aliados se empujan. En un beat 'em up cooperativo suele ser sí,
  con menos fuerza, para evitar solapamientos.
- Los enemigos que están cayendo o muertos pueden dejar de colisionar.

### Alternativa: empuje solo al jugador

Si te resulta frustrante que los enemigos te desplacen, puedes empujar solo al
enemigo y tratar al jugador como «pesado». Es una decisión de diseño; lo
importante es que sea consistente y se sienta bien.

## 6. Detección de golpes

La detección ocurre solo en los ticks activos del ataque:

```go
func (w *World) collectHits() []Hit {
	var hits []Hit
	for _, attacker := range w.Entities {
		if !attacker.Alive || attacker.State != StateAttack || attacker.Move == nil {
			continue
		}
		if !attacker.Move.IsActive(attacker.MoveTick) {
			continue
		}
		for _, target := range w.Entities {
			if target == attacker || !target.Alive || target.Team == attacker.Team {
				continue
			}
			if _, ok := attacker.HitTargets[target.ID]; ok {
				continue // ya golpeado por este ataque
			}
			hb := attacker.Move.Hitbox
			if !hb.World(attacker).Overlaps(target.Hurtbox()) {
				continue
			}
			if !overlapsZ(attacker, hb, target) {
				continue
			}
			hits = append(hits, Hit{Attacker: attacker, Target: target, Move: attacker.Move})
		}
	}
	return hits
}
```

Puntos clave:

- `HitTargets` se reinicia al empezar un ataque nuevo, no al cambiar de frame.
- Un ataque con dos hitboxes (puñetazo y patada) puede golpear a dos enemigos,
  pero cada uno una sola vez. Si quieres golpes multi-hit, usa una clave
  `(targetID, hitIndex)` o incrementa un contador de hits permitidos.
- La hitbox y la hurtbox se comprueban en el plano y en `Z`.
- No apliques daño aquí: recopila y resuelve después (capítulo 05).

## 7. Debug visual

Dibuja las cajas en espacio de pantalla, después de la cámara:

```go
func drawFloorBox(dst *ebiten.Image, b AABB, cam Camera, clr color.Color) {
	x := float32(b.MinX - cam.X)
	y := float32(b.MinY - cam.Y)
	w := float32(b.MaxX - b.MinX)
	h := float32(b.MaxY - b.MinY)
	vector.StrokeRect(dst, x, y, w, h, 1, clr, false)
}
```

Códigos de color útiles:

| Caja | Color |
|---|---|
| Cuerpo/hurtbox | azul |
| Hitbox activa | rojo |
| Pared/obstáculo | amarillo |
| Zona caminable | verde tenue |

Para `Z`, dibuja una barra vertical desde `(X, Y-Z)` hasta `(X, Y)` o un rectángulo
en la posición proyectada. No intentes renderizar un cubo 3D perfecto; con dos
vistas (planta y lateral) es suficiente para depurar.

## 8. Rendimiento y estructura

Con menos de 50 personajes, la comprobación `O(n²)` es despreciable. No añadas
un quadtree por moda. Si el juego crece:

1. Descarta por `X` antes de comprobar `Y` y `Z`.
2. Ordena por `X` y usa una ventana deslizante.
3. Solo entonces considera una rejilla espacial.

La detección de golpes ocurre únicamente en frames activos, así que el coste real
suele ser menor de lo que parece.

## 9. Errores frecuentes

| Síntoma | Causa probable |
|---|---|
| Los golpes no conectan aunque se toquen | Hitbox relativa mal espejada o `Z` mal sumado |
| El personaje atraviesa paredes | Resolución en los dos ejes a la vez o tunneling |
| Se queda pegado a la pared | No resolver por ejes separados |
| Los personajes se solapan | Falta separación o se aplica antes del movimiento |
| La diagonal es más rápida | Falta normalizar el vector |
| El salto golpea a quien está detrás | `Z` ignorado en la hitbox |
| El enemigo tiembla al empujar | Dos empujes por tick o falta de epsilon |
| La sombra flota | Se usó `Z` como posición de suelo |

## 10. Ejercicio del capítulo

1. Implementa `AABB`, `Body`, `BodyBox`, `Hurtbox` y `Hitbox.World`.
2. Haz que el jugador camine en 4 direcciones, con diagonal normalizada.
3. Limita la posición a una banda caminable y dibuja sus bordes.
4. Añade un obstáculo rectangular y comprueba que se desliza por él.
5. Añade dos enemigos y sepáralos con empuje suave.
6. Implementa un ataque con hitbox activa durante 2 ticks y dibuja la caja.
7. Comprueba con debug que un salto (`Z`) no golpea a un enemigo en el suelo.
8. Escribe tests de `Overlaps`, normalización y `clampToBounds`.

Criterio de aceptación: puedes predecir si un golpe conecta mirando las cajas
dibujadas, y el jugador no atraviesa paredes ni se queda pegado.

## 11. Referencias

- `geom.go` para transformaciones (no para colisiones: usa tu propia matemática).
- `vector` para dibujar cajas de debug.
- `examples/platformer` para movimiento y gravedad básicos.
- `examples/isometric` si más adelante quieres cambiar la proyección.
