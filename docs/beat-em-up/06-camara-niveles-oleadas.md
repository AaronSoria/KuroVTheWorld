# 06 — Cámara, niveles y oleadas

Un beat 'em up se juega en un escenario que avanza a la derecha, se detiene en
ciertos puntos y se cierra hasta que el jugador limpia la zona. La cámara no es
solo seguimiento: marca el ritmo, oculta y revela enemigos y delimita el espacio
de combate.

## 1. Cámara

```go
type Camera struct {
	X, Y float64 // esquina superior izquierda del mundo visible
	W, H float64 // tamaño del viewport lógico

	MinX, MaxX float64
	MinY, MaxY float64

	Locked bool
	LockX  float64

	ShakeX, ShakeY  float64
	ShakeTicks      int
	ShakeStrength   float64
}
```

Proyección:

```go
func (c *Camera) WorldToScreen(x, y, z float64) (float64, float64) {
	return x - c.X, y - z - c.Y
}
```

### Seguimiento con zona muerta

Una cámara que sigue al jugador píxel a píxel marea. Usa una zona muerta: el
jugador puede moverse un poco sin que la cámara reaccione.

```go
func (c *Camera) Follow(target *Entity, smoothing float64) {
	if c.Locked {
		c.X += (c.LockX - c.X) * smoothing
		c.clamp()
		return
	}

	desired := target.Pos.X - c.W*0.5
	// Zona muerta horizontal.
	left := c.X + c.W*0.5 - 48
	right := c.X + c.W*0.5 + 48
	if target.Pos.X < left {
		desired = target.Pos.X - (c.W*0.5 - 48)
	} else if target.Pos.X > right {
		desired = target.Pos.X - (c.W*0.5 + 48)
	} else {
		desired = c.X
	}

	c.X += (desired - c.X) * smoothing
	c.clamp()
}
```

`clamp` limita a los bordes del nivel:

```go
func (c *Camera) clamp() {
	if c.MaxX-c.MinX <= c.W {
		c.X = (c.MinX + c.MaxX - c.W) / 2
	} else {
		c.X = math.Max(c.MinX, math.Min(c.MaxX-c.W, c.X))
	}
	// Igual para Y si el nivel tiene profundidad mayor que el viewport.
}
```

Para un beat 'em up de scroll horizontal, `Y` suele ser fija o con un rango
pequeño. Si la cámara sigue la profundidad, el escenario «respira» y puede
marear; valora si lo necesitas.

### Suavizado

```go
const cameraSmoothing = 0.12
```

Demasiado suave (0.02) y la cámara va con retraso; demasiado brusco (1.0) y se
pega al jugador. Ajusta con el movimiento real, no con el número.

### Bloqueo de cámara

Durante una oleada, la cámara se detiene y el jugador no puede salir de la zona:

```go
func (w *World) LockCameraAt(x float64) {
	w.Camera.Locked = true
	w.Camera.LockX = x
}
```

Además de bloquear la cámara, añade una pared invisible en el borde derecho (y
quizá izquierdo) para que el jugador no se salga del encuadre. La pared debe
desaparecer al limpiar la oleada.

### Sacudida

La sacudida se aplica **al dibujar**, no a la lógica:

```go
func (c *Camera) Shake(ticks int, strength float64) {
	c.ShakeTicks = max(c.ShakeTicks, ticks)
	c.ShakeStrength = max(c.ShakeStrength, strength)
}

func (c *Camera) UpdateShake() {
	if c.ShakeTicks <= 0 {
		c.ShakeX, c.ShakeY = 0, 0
		return
	}
	c.ShakeTicks--
	// Movimiento determinista y decreciente.
	t := float64(c.ShakeTicks)
	c.ShakeX = math.Sin(t*1.7) * c.ShakeStrength * (t / 10)
	c.ShakeY = math.Cos(t*2.3) * c.ShakeStrength * (t / 10)
}
```

Reglas:

- No sacudas en cada golpe; reserva la sacudida para impactos fuertes, jefes o
  explosiones.
- Ofrece una opción para reducir o desactivar la sacudida.
- No cambies `Camera.X` con la sacudida; si lo haces, las colisiones y el HUD
  quedan desincronizados.

## 2. Fondos y parallax

Capas típicas:

| Capa | Factor | Contenido |
|---|---|---|
| Cielo | 0 | Gradiente o color fijo |
| Lejos | 0.1–0.3 | Edificios, montañas |
| Medio | 0.4–0.7 | Vallas, carteles, árboles |
| Suelo | 1.0 | Acera, calzada, decorado jugable |
| Delante | 1.1–1.3 | Plantas, columnas que tapan parcialmente |

```go
type ParallaxLayer struct {
	Image  *ebiten.Image
	Factor float64
	Y      float64
}

func (l *ParallaxLayer) Draw(dst *ebiten.Image, cam Camera) {
	op := &ebiten.DrawImageOptions{}
	x := -cam.X * l.Factor
	// Repite horizontalmente si la imagen es más estrecha que el nivel.
	for x < float64(ScreenWidth) {
		op.GeoM.Reset()
		op.GeoM.Translate(math.Round(x), l.Y)
		dst.DrawImage(l.Image, op)
		x += float64(l.Image.Bounds().Dx())
	}
}
```

Detalles:

- Redondea la posición para evitar temblor.
- Si la capa se repite, asegúrate de que el borde izquierdo y derecho coinciden.
- El suelo se dibuja con factor 1.0 y debe cubrir toda la zona caminable.
- No dibujes capas fuera de la pantalla; el `for` anterior ya lo evita.
- Para fondos muy grandes, divide en tiles y dibuja solo los visibles.

## 3. Datos del nivel

Define el nivel como datos, no como código:

```go
type Level struct {
	Name         string
	Music        string
	Bounds       Bounds
	CameraBounds Bounds
	Waves        []Wave
	Props        []Prop
	ExitX        float64
}

type Wave struct {
	ID             string
	Trigger        Trigger
	LockCamera     bool
	CameraLockX    float64
	Spawns         []Spawn
	MaxAlive       int
	SpawnInterval  int // ticks entre spawns
	NextWhenClear  bool
}

type Trigger struct {
	PlayerX   float64
	AfterWave string
	Delay     int // ticks
}

type Spawn struct {
	Kind string
	X, Y float64
}

type Prop struct {
	Kind string
	X, Y float64
}
```

JSON de ejemplo:

```json
{
  "name": "street01",
  "music": "stage1",
  "bounds": { "minX": 0, "maxX": 3200, "minY": 180, "maxY": 300 },
  "cameraBounds": { "minX": 0, "maxX": 3200, "minY": 0, "maxY": 270 },
  "waves": [
    {
      "id": "w1",
      "trigger": { "playerX": 300 },
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
    },
    {
      "id": "w2",
      "trigger": { "playerX": 1200, "afterWave": "w1" },
      "lockCamera": true,
      "cameraLockX": 1140,
      "maxAlive": 3,
      "spawnInterval": 60,
      "nextWhenClear": true,
      "spawns": [
        { "kind": "thug", "x": 1450, "y": 220 },
        { "kind": "thug", "x": 1500, "y": 280 },
        { "kind": "bruiser", "x": 1600, "y": 250 }
      ]
    }
  ],
  "exitX": 3100
}
```

Ventajas:

- Cambiar el nivel no requiere recompilar.
- Puedes validar los datos al cargar (bounds, spawns dentro de la zona, oleadas
  con IDs únicos).
- Los tests pueden usar un nivel mínimo sin assets.

Carga y valida:

```go
func LoadLevel(path string) (*Level, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var l Level
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, err
	}
	if err := l.Validate(); err != nil {
		return nil, err
	}
	return &l, nil
}
```

No dejes que un JSON mal escrito cree un enemigo en `(0,0)` o fuera del escenario;
falla al cargar, no en mitad de la partida.

## 4. Oleadas

### Máquina de estados

```go
type WaveState int

const (
	WaveWaiting WaveState = iota
	WaveSpawning
	WaveFighting
	WaveCleared
	WaveDone
)

type WaveRunner struct {
	Wave    *Wave
	State   WaveState
	Index   int // siguiente spawn
	Timer   int
	Alive   int
	Delay   int
}
```

Flujo:

```text
Waiting  --trigger--> Spawning
Spawning --spawn y espera--> Spawning / Fighting
Fighting --quedan enemigos--> Fighting
Fighting --Alive == 0 y todos spawned--> Cleared
Cleared  --siguiente trigger--> Waiting / Done
```

### Actualización

```go
func (r *WaveRunner) Update(w *World) {
	switch r.State {
	case WaveWaiting:
		if r.TriggerMet(w) {
			r.State = WaveSpawning
			r.Timer = 0
			if r.Wave.LockCamera {
				w.LockCameraAt(r.Wave.CameraLockX)
			}
		}

	case WaveSpawning:
		if r.Index >= len(r.Wave.Spawns) {
			r.State = WaveFighting
			return
		}
		if r.Alive >= r.Wave.MaxAlive {
			return
		}
		if r.Timer > 0 {
			r.Timer--
			return
		}
		w.Spawn(r.Wave.Spawns[r.Index])
		r.Index++
		r.Timer = r.Wave.SpawnInterval
		if r.Index >= len(r.Wave.Spawns) {
			r.State = WaveFighting
		}

	case WaveFighting:
		r.Alive = w.CountAlive(TeamEnemy)
		if r.Alive == 0 {
			r.State = WaveCleared
		}

	case WaveCleared:
		if r.Wave.LockCamera {
			w.UnlockCamera()
		}
		r.State = WaveDone
	}
}
```

`Alive` debe contar enemigos vivos, no solo spawneados. Un enemigo que está
muriendo pero aún se ve no debería bloquear la oleada para siempre; decide si
cuenta hasta que termina la animación o hasta que `Alive=false`. Lo habitual es
contar hasta que desaparece.

### Spawns justos

- **Fuera de la pantalla o en el borde**: no hagas aparecer un enemigo encima del
  jugador.
- **Profundidad válida**: `Y` dentro de `Bounds`, no en una pared.
- **Sin solapamiento**: comprueba que la caja no se solapa con otro cuerpo; si
  pasa, reintenta en otro punto o espera.
- **Máximo de vivos**: limita la presión simultánea.
- **Intervalo**: da tiempo a reaccionar entre apariciones.
- **Entrada desde el fondo o los lados**: coherente con el escenario. Un enemigo
  que sale de una puerta debe venir de esa puerta.

```go
func (w *World) Spawn(s Spawn) *Entity {
	e := w.Pool.Get()
	e.Kind = KindFromString(s.Kind)
	e.Team = TeamEnemy
	e.Pos = Vec3{X: s.X, Y: s.Y}
	e.Alive = true
	// ... vida, animación, IA
	w.Entities = append(w.Entities, e)
	return e
}
```

### Triggers

Un trigger puede ser:

- `PlayerX`: el jugador cruza una coordenada.
- `AfterWave`: la oleada anterior terminó.
- `Delay`: ticks desde que se cumple la condición.
- Combinación de los tres.

Evita triggers que dependan solo de un temporizador global: si el jugador se
entretiene, la siguiente oleada puede aparecer antes de tiempo. La condición
`AfterWave` + `PlayerX` es más robusta.

### Nivel terminado

Cuando la última oleada está limpia:

1. Desbloquea la cámara.
2. Abre la salida (quita la pared invisible).
3. Muestra una flecha o mensaje.
4. Al cruzar `ExitX`, transición a la siguiente escena o nivel.

No mezcles «oleada limpia» con «nivel terminado»: son dos estados distintos.

## 5. Pooling de entidades

Crear y destruir decenas de enemigos por nivel es manejable, pero un pool evita
asignaciones y bugs de referencias:

```go
type EntityPool struct {
	free []*Entity
}

func (p *EntityPool) Get() *Entity {
	if n := len(p.free); n > 0 {
		e := p.free[n-1]
		p.free = p.free[:n-1]
		return e
	}
	return &Entity{}
}

func (p *EntityPool) Put(e *Entity) {
	*e = Entity{} // limpia TODOS los campos
	p.free = append(p.free, e)
}
```

Peligro: si olvidas limpiar un campo (`HitTargets`, `Move`, `Anim.Current`,
`Hurt`, `Invuln`), el enemigo reutilizado hereda estado. Escribe un `Reset()`
explícito y testéalo. Si el juego es pequeño, no uses pool; la claridad vale más.

## 6. Debug de cámara y oleadas

Muestra en pantalla:

```text
cam: (x, y)  lock: sí/no
oleada: w2  estado: Fighting  vivos: 2/3  spawn: 1/3
jugador: (x, y)  bounds: [...]
```

Dibuja:

- El rectángulo visible de la cámara en el minimapa.
- Las coordenadas de trigger como líneas verticales.
- La zona caminable.
- Los puntos de spawn.
- El estado de cada enemigo.

Con eso, la mayoría de los bugs de «no aparece la oleada» se resuelven en
segundos.

## 7. Errores frecuentes

| Síntoma | Causa probable |
|---|---|
| La cámara se sale del nivel | Falta `clamp` a `CameraBounds` |
| El jugador sale del encuadre | Falta pared invisible al bloquear cámara |
| La oleada no termina | Se cuentan enemigos muertos o no spawneados |
| Aparecen enemigos encima del jugador | Spawn sin comprobar intersección |
| El fondo tiembla | Posición de parallax sin redondear |
| El nivel se ve borroso | Escala no entera o capas desalineadas |
| La sacudida descoloca el HUD | Se aplicó a la cámara lógica |
| Se puede saltar una oleada | Trigger sin pared o sin `AfterWave` |

## 8. Ejercicio del capítulo

1. Implementa `Camera` con seguimiento, zona muerta, clamp y bloqueo.
2. Añade parallax con tres capas y comprueba que no tiemblan.
3. Define `Level`, `Wave`, `Spawn` y cárgalos desde JSON.
4. Implementa `WaveRunner` con `Waiting → Spawning → Fighting → Cleared`.
5. Limita a 2 enemigos vivos y añade intervalo entre spawns.
6. Bloquea la cámara al empezar la oleada y desbloquéala al limpiarla.
7. Añade una salida que solo se activa tras la última oleada.
8. Escribe un test del `WaveRunner` con un `World` falso y comprueba las
   transiciones sin abrir ventana.
9. Dibuja los triggers y la zona caminable en modo debug.

Criterio de aceptación: puedes recorrer el nivel, provocar las tres oleadas, ver
que la cámara se bloquea y se libera, y terminar el nivel sin tocar el código.

## 9. Referencias

- `examples/camera` para una cámara con transformación.
- `examples/infinitescroll` para fondos repetidos.
- `examples/platformer` para scroll y gravedad.
- `encoding/json` y `//go:embed` para niveles.
- `image.Rectangle` para bounds y validación.
