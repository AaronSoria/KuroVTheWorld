# 02 — Arquitectura de un beat 'em up

Un beat 'em up pequeño puede vivir en un solo archivo, pero en cuanto hay combate,
oleadas, menús y guardado, el desorden se convierte en el principal enemigo. Este
capítulo propone una arquitectura mínima, explícita y fácil de testear. No es la
única válida; sí es una que escala sin obligarte a reescribirlo todo.

## 1. Estructura de paquetes

```text
mi-beatemup/
├── cmd/
│   └── game/
│       └── main.go          # solo arranca el juego
├── internal/
│   └── game/
│       ├── game.go          # Game, escenas, bucle
│       ├── scene_title.go
│       ├── scene_play.go
│       ├── input.go         # adaptador Ebitengine -> InputState
│       ├── world.go         # entidades, actualización, colisiones
│       ├── entity.go        # Entity, estados, equipos
│       ├── player.go        # control del jugador
│       ├── enemy.go         # IA
│       ├── combat.go        # golpes, daño, hitstop
│       ├── camera.go
│       ├── level.go         # carga de nivel y oleadas
│       ├── hud.go
│       ├── assets.go        # carga única de imágenes/fuentes/audio
│       └── data.go          # structs de moves, niveles, ajustes
├── assets/
│   ├── sprites/
│   ├── audio/
│   └── fonts/
└── levels/
    └── street01.json
```

`cmd/game/main.go` no debería contener lógica:

```go
package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"ejemplo.com/mi-beatemup/internal/game"
)

func main() {
	g, err := game.New()
	if err != nil {
		log.Fatal(err)
	}
	ebiten.SetWindowSize(game.ScreenWidth*2, game.ScreenHeight*2)
	ebiten.SetWindowTitle("Mi beat 'em up")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
```

`internal/` impide que otros módulos importen tus tripas. Si más adelante quieres
un editor o herramientas, crea `cmd/editor` y comparte paquetes en `internal/`.

## 2. Escenas

Una escena es una pantalla con su propio `Update` y `Draw`: título, juego, pausa,
game over, opciones. La forma más simple:

```go
type Scene interface {
	// Update devuelve la siguiente escena, o nil para quedarse en la actual.
	Update() (Scene, error)
	Draw(screen *ebiten.Image)
}
```

```go
type Game struct {
	scene Scene
}

func (g *Game) Update() error {
	next, err := g.scene.Update()
	if err != nil {
		return err
	}
	if next != nil {
		g.scene = next
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.scene.Draw(screen)
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ScreenWidth, ScreenHeight
}
```

Ventajas: transiciones explícitas, sin punteros globales. Para pausa hay dos
opciones:

1. **Bandera dentro de la escena de juego**: `paused bool`. Simple, suficiente
   para el primer vertical slice. El mundo no se actualiza, pero el menú sí.
2. **Pila de escenas**: `PauseScene` envuelve o guarda la escena anterior y la
   dibuja congelada debajo. Más flexible para menús superpuestos.

No construyas un sistema de escenas genérico con eventos y dependencias antes de
tener dos escenas reales. Empieza con la bandera.

### Ejemplo de escena de título

```go
type TitleScene struct {
	next Scene
}

func (s *TitleScene) Update() (Scene, error) {
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) ||
		inpututil.IsStandardGamepadButtonJustPressed(0, ebiten.StandardGamepadButtonRightBottom) {
		return NewPlayScene(), nil
	}
	return nil, nil
}

func (s *TitleScene) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0x10, 0x10, 0x18, 0xff})
	// texto y prompt
}
```

## 3. Entidades: composición, no jerarquías profundas

Un beat 'em up tiene pocos tipos de entidad (jugador, enemigos, objetos, efectos).
No necesitas un ECS completo. Un struct con datos y unos pocos sistemas basta.

```go
type Vec3 struct {
	X float64 // posición lateral, + derecha
	Y float64 // profundidad del suelo, + hacia la cámara
	Z float64 // altura sobre el suelo, + arriba
}

type Team int

const (
	TeamPlayer Team = iota
	TeamEnemy
)

type Kind int

const (
	KindPlayer Kind = iota
	KindThug
	KindBruiser
	KindBoss
)

type State int

const (
	StateIdle State = iota
	StateWalk
	StateJump
	StateAttack
	StateBlock
	StateHurt
	StateDown
	StateGetUp
	StateDead
)

type Entity struct {
	ID   int
	Kind Kind
	Team Team

	Pos    Vec3
	Vel    Vec3
	Facing int // +1 derecha, -1 izquierda

	// Caja del cuerpo en el plano del suelo (X/Y). Z se gestiona aparte.
	Body Box

	State     State
	StateTick int // ticks desde que empezó el estado actual

	Health    int
	MaxHealth int

	WalkSpeedX float64 // px/tick
	WalkSpeedY float64

	Anim Animator

	// Ataque actual (nil si no está atacando).
	Move       *Move
	MoveTick   int
	MoveHit    bool
	HitTargets map[int]struct{} // IDs ya golpeados por este ataque

	Blocking    bool
	Blockstun   int
	Hitstop     int // ticks congelado por impacto
	Invuln      int // ticks de invulnerabilidad
	Hurt        int // ticks restantes de hitstun
	JuggleCount int

	ComboCount int
	ComboTimer int

	SolidWithAllies bool
	Alive           bool
}
```

Decisiones importantes:

- **`Y` es profundidad, no altura.** El suelo es un plano `X/Y`; `Z` es altura.
  La posición en pantalla se calcula como `(X, Y - Z)`. Esto simplifica saltos,
  proyectiles y sombras.
- **`Facing` es un entero `+1/-1`.** Evita ramas con floats y facilita espejar.
- **`Body` es la caja física**, distinta de las hitboxes de ataque.
- **`HitTargets` pertenece al ataque**, no a la entidad: evita golpear dos veces
  al mismo enemigo con el mismo golpe.
- **`Alive` en lugar de borrar durante el recorrido.** Compacta al final del tick.

No metas punteros a `*ebiten.Image` en `Entity` si puedes usar un identificador o
un `*Sprite` compartido. Las imágenes son recursos globales, no estado de una
entidad.

## 4. El mundo y el orden de actualización

```go
type World struct {
	Tick     int64
	Player   *Entity
	Entities []*Entity // incluye al jugador
	Camera   Camera
	Level    *Level
	RNG      *rand.Rand
	Hitstop  int // hitstop global opcional
}
```

El orden importa. Un orden probado:

```text
1. Leer input y actualizar el buffer.
2. Actualizar temporizadores globales (hitstop, cámara, efectos).
3. Para cada entidad: decidir intención (jugador o IA).
4. Para cada entidad: aplicar velocidad y resolver movimiento/colisiones.
5. Detectar golpes activos contra hurtboxes.
6. Resolver los golpes recopilados (daño, hitstun, knockback, hitstop).
7. Actualizar estados, animaciones y temporizadores.
8. Actualizar cámara y oleadas/triggers.
9. Eliminar entidades muertas y efectos expirados.
```

¿Por qué separar 5 y 6? Porque permite resolver intercambios de forma
determinista: primero se recogen todos los impactos del tick y luego se aplican.
Si aplicas daño mientras recorres, el resultado depende del orden del slice.

Reglas:

- No modifiques `Entities` mientras la recorres. Marca `Alive = false` y compacta
  al final.
- No dependas del orden de un `map` para decisiones de juego. Itera slices
  ordenados por `ID`.
- Un sistema debe ser idempotente por tick: llamar dos veces a `Update` no debe
  duplicar daño.
- Si una entidad muere, no la elimines inmediatamente: deja que termine su
  animación de muerte y que los efectos / HUD la consulten.

### Compactar sin basura

```go
func (w *World) cleanup() {
	dst := w.Entities[:0]
	for _, e := range w.Entities {
		if e.Alive {
			dst = append(dst, e)
		}
	}
	// Limpia la cola para no retener punteros.
	for i := len(dst); i < len(w.Entities); i++ {
		w.Entities[i] = nil
	}
	w.Entities = dst
}
```

Esto reutiliza el array subyacente y no asigna un slice nuevo cada tick.

### Separar simulación de Ebitengine

Para poder testear, el mundo no debería llamar a `ebiten.IsKeyPressed`. Define un
input propio:

```go
type InputState struct {
	MoveX, MoveY float64 // [-1, 1]
	Attack       bool
	Heavy        bool
	Jump         bool
	Block        bool
	Pause        bool

	// Bordes: true solo en el tick en que se pulsó.
	AttackPressed bool
	JumpPressed   bool

	// Navegación de menús (bordes y repetición gestionados aparte).
	MenuUp, MenuDown   bool
	MenuLeft, MenuRight bool
	ConfirmPressed     bool
	CancelPressed      bool
}
```

El adaptador vive en `input.go` y traduce teclado/gamepad. Así puedes probar
`world.Update(InputState{...})` en un test sin abrir ventana.

## 5. Sistemas pequeños y explícitos

No hace falta una clase `System`. Funciones con receptor de mundo son suficientes:

```go
func (w *World) updatePlayer(in InputState) { /* intención y estados */ }
func (w *World) updateEnemies()            { /* IA */ }
func (w *World) integrateMovement()        { /* posición y colisiones */ }
func (w *World) collectHits() []Hit        { /* hitboxes activas */ }
func (w *World) applyHits(hits []Hit)      { /* daño y reacciones */ }
func (w *World) updateCamera()             { /* seguimiento */ }
func (w *World) updateWaves()              { /* spawns y triggers */ }
```

Ventajas: se leen de arriba abajo, se testean por separado y no hay magia de
reflexión. Cuando un sistema crezca, extráelo a su archivo.

### Búsqueda por ID

```go
func (w *World) ByID(id int) *Entity {
	for _, e := range w.Entities {
		if e.ID == id {
			return e
		}
	}
	return nil
}
```

Con menos de 100 entidades, la búsqueda lineal es irrelevante. Si necesitas
consultas frecuentes, añade un `map[int]*Entity` y mantenlo sincronizado; no
optimices antes de medir.

## 6. Espacios de coordenadas

Hay tres espacios y conviene nombrarlos sin ambigüedad:

| Espacio | Uso | Transformación |
|---|---|---|
| Mundo | Posiciones, cajas, velocidades | — |
| Pantalla | Dibujo del mundo | `screen = world - camera` |
| UI | HUD, menús, texto | Sin cámara; píxeles lógicos |

```go
type Camera struct {
	X, Y float64 // esquina superior izquierda del mundo visible
}

func (c Camera) WorldToScreen(x, y, z float64) (float64, float64) {
	return x - c.X, y - z - c.Y
}
```

Dibuja el mundo con la cámara aplicada y el HUD después, sin cámara:

```go
func (s *PlayScene) Draw(screen *ebiten.Image) {
	screen.Fill(backgroundColor)
	s.world.Draw(screen) // aplica cámara
	s.world.DrawHUD(screen)
}
```

Si usas un offscreen para efectos, recuerda que el HUD no debe pasar por él (o
sí, según el efecto). Decide y documéntalo.

### Redondeo de píxeles

```go
func snap(v float64) float64 {
	return math.Round(v)
}
```

Aplícalo al dibujar, no a la simulación. Si redondeas la posición lógica, pierdes
precisión y las colisiones se vuelven dependientes de la escala. Redondea la
cámara y las posiciones de pantalla en el momento de dibujar.

## 7. Datos, no constantes dispersas

Los números que vas a ajustar (velocidad, daño, frames, alcance) deben estar en
un sitio:

```go
type Tuning struct {
	PlayerWalkSpeed float64 // px/tick
	PlayerDepthSpeed float64
	PlayerJumpSpeed float64
	Gravity         float64
}

var DefaultTuning = Tuning{
	PlayerWalkSpeed:  2.5,
	PlayerDepthSpeed: 2.0,
	PlayerJumpSpeed:  6.0,
	Gravity:          0.35,
}
```

Mejor aún: un `Move` por ataque en JSON o en un mapa de datos (capítulo 05). Lo
importante es que un diseñador (tú, dentro de un mes) pueda cambiar un número sin
buscar en diez archivos.

## 8. Carga de recursos

Carga todo al arrancar y falla rápido si falta algo:

```go
type Assets struct {
	PlayerSheet *ebiten.Image
	EnemySheet  *ebiten.Image
	Font        *text.GoTextFaceSource
	HitSound    []byte
}

func LoadAssets() (*Assets, error) {
	player, err := decodeImage(playerPNG)
	if err != nil {
		return nil, fmt.Errorf("player: %w", err)
	}
	return &Assets{PlayerSheet: player}, nil
}
```

- Un asset se carga una vez y se comparte; nunca lo modifiques por entidad.
- Si una animación necesita un frame concreto, usa `SubImage` (comparte píxeles).
- Las fuentes y los reproductores de audio también son recursos, no estado.
- Para hot reload en desarrollo, puedes recargar desde disco con una tecla, pero
  no lo mezcles con la ruta de producción.

## 9. Errores frecuentes

| Síntoma | Causa probable |
|---|---|
| Un enemigo recibe daño dos veces | No llevar `HitTargets` por ataque |
| El juego cambia según el orden de spawn | Resolver daño dentro del recorrido |
| Punteros a entidades muertas | No compactar o no limpiar referencias |
| Todo se actualiza dos veces | `Update` llamado desde varios sitios |
| El HUD se mueve con la cámara | Aplicar la cámara también a la UI |
| Tests imposibles de escribir | Lógica acoplada a `ebiten.IsKeyPressed` |
| Se pierde el input al pausar | No actualizar el buffer o consumirlo dos veces |

## 10. Ejercicio del capítulo

1. Crea la estructura de paquetes anterior con `cmd/game` y `internal/game`.
2. Implementa `Game`, `Scene`, `TitleScene` y `PlayScene` con transición.
3. Define `Vec3`, `Entity`, `World` y el orden de `Update` con funciones vacías.
4. Haz que `PlayScene` cree un jugador y un enemigo y los dibuje como cajas.
5. Añade un `InputState` y un adaptador que lea teclado y gamepad.
6. Escribe un test que cree un `World`, llame a `Update` con un input y compruebe
   que la posición cambió. El test no debe abrir ventana.
7. Añade un contador de entidades y un toggle de debug.

Criterio de aceptación: puedes cambiar el input del jugador en un test sin tocar
Ebitengine, y las escenas no se conocen entre sí más que por la interfaz.

## 11. Referencias

- `examples/2048` para separar `Game`, `Board` e `Input`.
- `examples/platformer` para un bucle mínimo.
- `ebiten.Game` y `RunGame` en `run.go`.
- `image.go` para el ciclo de vida de las imágenes.
