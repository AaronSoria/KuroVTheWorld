# 05 — Combate

El combate es el corazón del género. Un beat 'em up se siente bien cuando cada
golpe tiene peso, cada ataque se puede leer y el jugador entiende por qué ha
acertado o fallado. La implementación se apoya en tres ideas: **frame data**,
**máquina de estados** y **resolución determinista de impactos**.

## 1. Frame data: el vocabulario

Cada ataque se describe con ticks (a 60 TPS):

| Fase | Qué ocurre | Se puede cancelar |
|---|---|---|
| `Startup` | El personaje se prepara; la hitbox aún no existe | Normalmente no |
| `Active` | La hitbox existe y puede golpear | Sí, según la técnica |
| `Recovery` | El personaje se recupera; ya no golpea | Sí, al golpear |

Ejemplo: `startup=4, active=2, recovery=8` significa:

```text
tick:  0  1  2  3  4  5  6 ... 13
       [-- startup --][act][-- recovery --]
```

- En los ticks 4 y 5 la hitbox está activa.
- En el tick 6 empieza la recuperación.
- En el tick 14 el ataque termina (`4+2+8`).

A 60 TPS, 4 ticks son unos 67 ms. Un golpe ligero suele tener 3–6 ticks de
startup; uno pesado, 10–20. Si todos los ataques son rápidos, el juego se siente
sin peso; si todos son lentos, se siente injusto.

### Ventaja tras el golpe

Si un ataque tiene `hitstun=14` y `recovery=8`, tras conectar quedan 6 ticks de
ventaja: el atacante puede actuar antes de que la víctima salga del hitstun. Esa
ventaja es la base de los combos. Si `recovery > hitstun`, el atacante está en
desventaja y puede ser castigado.

No copies números de otro juego sin entender esta relación. Ajusta con una tabla
y pruébalo.

## 2. Estructura de un movimiento

```go
type Move struct {
	Name string
	Anim string

	Startup  int
	Active   int
	Recovery int

	Damage    int
	Hitstun   int
	Blockstun int
	Hitstop   int

	// Impulso aplicado a la víctima, en px/tick.
	KnockbackX float64
	KnockbackY float64

	Hitbox Hitbox

	// Tick desde el que se puede cancelar en otro movimiento (si acierta).
	CancelFrom int
	Next       []string // nombres de movimientos encadenables

	// Opcionales
	Unblockable bool
	ChipDamage  int
	JuggleLimit int
}

func (m *Move) Total() int {
	return m.Startup + m.Active + m.Recovery
}

func (m *Move) IsActive(tick int) bool {
	return tick >= m.Startup && tick < m.Startup+m.Active
}
```

Guarda estos datos en un mapa o en JSON:

```go
var Moves = map[string]*Move{
	"jab": {
		Name: "jab", Anim: "attack_1",
		Startup: 4, Active: 2, Recovery: 8,
		Damage: 6, Hitstun: 14, Blockstun: 8, Hitstop: 4,
		KnockbackX: 1.5, KnockbackY: 0,
		Hitbox: Hitbox{OffsetX: 18, OffsetY: 0, Width: 24, Depth: 18, ZMin: 20, ZMax: 50},
		CancelFrom: 6, Next: []string{"jab2"},
	},
	"jab2": {
		Name: "jab2", Anim: "attack_2",
		Startup: 6, Active: 2, Recovery: 10,
		Damage: 8, Hitstun: 16, Blockstun: 10, Hitstop: 5,
		KnockbackX: 2.0, KnockbackY: 0,
		Hitbox: Hitbox{OffsetX: 20, OffsetY: 0, Width: 26, Depth: 18, ZMin: 20, ZMax: 50},
		CancelFrom: 10, Next: []string{"finisher"},
	},
	"finisher": {
		Name: "finisher", Anim: "attack_3",
		Startup: 10, Active: 3, Recovery: 18,
		Damage: 14, Hitstun: 24, Blockstun: 14, Hitstop: 8,
		KnockbackX: 4.0, KnockbackY: 0,
		Hitbox: Hitbox{OffsetX: 24, OffsetY: 0, Width: 32, Depth: 22, ZMin: 10, ZMax: 55},
	},
}
```

No hace falta que el primer prototipo tenga tres ataques; empieza con uno y añade
el resto cuando el pipeline funcione.

## 3. Máquina de estados

Estados mínimos:

```go
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
```

Transiciones:

| Desde | Evento | Hacia |
|---|---|---|
| `Idle`, `Walk` | input de ataque | `Attack` |
| `Idle`, `Walk` | input de salto | `Jump` |
| `Idle`, `Walk` | mantener atrás | `Block` |
| `Attack` | termina `Total()` | `Idle` |
| `Attack` | cancelación válida | otro `Attack` |
| cualquiera | recibe golpe | `Hurt` |
| `Hurt` | termina hitstun | `Idle` |
| `Hurt` | golpe fuerte / sin vida | `Down` o `Dead` |
| `Down` | termina caída | `GetUp` |
| `GetUp` | termina levantarse | `Idle` |
| `Dead` | termina animación | se elimina |

### Puede actuar

```go
func (e *Entity) CanAct() bool {
	switch e.State {
	case StateIdle, StateWalk:
		return true
	case StateAttack:
		return e.CanCancel()
	default:
		return false
	}
}
```

`CanCancel` comprueba que el ataque actual haya conectado, que estemos en la
ventana de cancelación y que el siguiente movimiento esté permitido:

```go
func (e *Entity) CanCancel() bool {
	if e.Move == nil || !e.MoveHit {
		return false
	}
	if e.MoveTick < e.Move.CancelFrom {
		return false
	}
	return true
}
```

`MoveHit` se pone a `true` cuando el ataque golpea a alguien. Si quieres permitir
cancelar al bloquear o al whiffear, cambia la condición explícitamente; no lo
dejes implícito.

### Iniciar un ataque

```go
func (e *Entity) StartAttack(m *Move) {
	e.State = StateAttack
	e.StateTick = 0
	e.Move = m
	e.MoveTick = 0
	e.MoveHit = false
	if e.HitTargets == nil {
		e.HitTargets = make(map[int]struct{}, 4)
	} else {
		clear(e.HitTargets)
	}
	e.Anim.Play(m.Anim, true)
}
```

`clear` reutiliza el mapa y evita asignar uno nuevo por ataque.

### Avanzar el ataque

Define el orden con precisión: **primero se detectan los golpes con el
`MoveTick` actual, después se incrementa**. Así el tick 0 es el primer tick de
startup y el tick `Startup` es el primero activo.

```go
func (e *Entity) AdvanceAttack() {
	if e.State != StateAttack || e.Move == nil {
		return
	}
	e.MoveTick++
	if e.MoveTick >= e.Move.Total() {
		e.State = StateIdle
		e.Move = nil
		e.MoveTick = 0
		e.Anim.Play("idle", true)
	}
}
```

Si incrementas antes de detectar, el ataque se adelanta un tick. Elige un orden,
documéntalo y testéalo.

## 4. Pipeline de un golpe

```text
1. Comprobar si el atacante está en un tick activo.
2. Comprobar hitbox contra hurtbox y Z.
3. Descartar objetivos ya golpeados por este ataque.
4. Recoger todos los impactos del tick.
5. Ordenar por (ID atacante, ID objetivo) para ser determinista.
6. Resolver cada impacto: bloqueo, daño, hitstun, knockback, hitstop.
7. Marcar el objetivo como golpeado.
8. Emitir efectos y sonidos.
9. Incrementar contadores de ataque/estado.
```

### Recoger

```go
type Hit struct {
	Attacker *Entity
	Target   *Entity
	Move     *Move
}

func (w *World) CollectHits() []Hit {
	var hits []Hit
	for _, a := range w.Entities {
		if !a.Alive || a.State != StateAttack || a.Move == nil {
			continue
		}
		if !a.Move.IsActive(a.MoveTick) {
			continue
		}
		for _, t := range w.Entities {
			if t == a || !t.Alive || t.Team == a.Team {
				continue
			}
			if _, ok := a.HitTargets[t.ID]; ok {
				continue
			}
			if !a.Move.Hitbox.World(a).Overlaps(t.Hurtbox()) {
				continue
			}
			if !overlapsZ(a, a.Move.Hitbox, t) {
				continue
			}
			hits = append(hits, Hit{Attacker: a, Target: t, Move: a.Move})
		}
	}
	return hits
}
```

### Resolver

```go
func (w *World) ResolveHits(hits []Hit) {
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Attacker.ID != hits[j].Attacker.ID {
			return hits[i].Attacker.ID < hits[j].Attacker.ID
		}
		return hits[i].Target.ID < hits[j].Target.ID
	})

	for _, h := range hits {
		a, t := h.Attacker, h.Target
		if !a.Alive || !t.Alive {
			continue
		}
		if _, ok := a.HitTargets[t.ID]; ok {
			continue
		}
		a.HitTargets[t.ID] = struct{}{}
		w.applyHit(h)
	}
}
```

Marcar el objetivo **aquí**, no en `CollectHits`, evita que una resolución
intermedia cambie el conjunto de impactos del tick. Si dos atacantes golpean al
mismo objetivo, ambos impactos se resuelven: es un intercambio válido.

### Aplicar

```go
func (w *World) applyHit(h Hit) {
	a, t, m := h.Attacker, h.Target, h.Move

	if t.Invuln > 0 || t.State == StateDead {
		return
	}

	blocked := t.CanBlock(a)
	if m.Unblockable {
		blocked = false
	}

	// Hitstop: congela a ambos, pero no la UI ni los efectos.
	a.Hitstop = max(a.Hitstop, m.Hitstop)
	t.Hitstop = max(t.Hitstop, m.Hitstop)

	if blocked {
		t.Health -= m.ChipDamage
		if t.Health <= 0 {
			t.Health = 0
			w.kill(t)
			return
		}
		t.State = StateBlock
		t.StateTick = 0
		t.Blockstun = m.Blockstun
		t.Vel.X = knockbackDir(a, t) * m.KnockbackX * 0.5
		t.Anim.Play("block", true)
		return
	}

	t.Health -= m.Damage
	t.Anim.Play("hurt", true)
	t.State = StateHurt
	t.StateTick = 0
	t.Hurt = m.Hitstun
	t.Vel.X = knockbackDir(a, t) * m.KnockbackX
	t.Vel.Y = m.KnockbackY

	a.MoveHit = true
	a.ComboCount++
	a.ComboTimer = 30

	if t.Health <= 0 {
		t.Health = 0
		w.kill(t)
	}
}

func knockbackDir(a, t *Entity) float64 {
	if t.Pos.X > a.Pos.X {
		return 1
	}
	if t.Pos.X < a.Pos.X {
		return -1
	}
	return float64(a.Facing)
}
```

`max` existe para enteros en Go 1.21+. Si usas una versión anterior, escribe el
`if` a mano.

## 5. Hitstop: el secreto del «peso»

El hitstop congela la animación y el movimiento durante unos ticks al conectar.
No es un adorno: da tiempo al jugador a leer el impacto y hace que el golpe se
sienta contundente.

Implementación por entidad:

```go
func (w *World) Update() {
	for _, e := range w.Entities {
		if e.Hitstop > 0 {
			e.Hitstop--
			continue // no mueve, no anima, no ataca
		}
		// ... movimiento, ataque, animación
	}
}
```

Reglas:

- El hitstop se aplica **después** de resolver el golpe, no antes.
- No acumules hitstop sin límite: `Hitstop = max(Hitstop, nuevo)` o un tope.
- La UI, la cámara, las partículas y el sonido siguen avanzando; si congelas todo,
  el juego parece roto.
- El hitstop no debe bloquear el buffer de input: el jugador puede seguir
  pulsando durante la pausa.
- Si congelas al atacante y a la víctima, el ataque no avanza; al terminar, ambos
  continúan. Si solo congelas a la víctima, el atacante puede cancelar antes de
  tiempo; elige una regla.

Para un impacto más global, puedes usar un `World.Hitstop` que detenga la
simulación entera. Es más fácil, pero los enemigos que no participan también se
congelan. Muchos juegos del género lo hacen así; pruébalo.

## 6. Hitstun, knockback y caída

- `Hurt` dura `Hitstun` ticks. Mientras dura, la entidad no puede actuar.
- El knockback se aplica como velocidad y se amortigua con fricción.
- Si el golpe es fuerte (o la vida llega a 0), pasa a `Down`.
- `Down` tiene una duración; después `GetUp`.
- `GetUp` concede invulnerabilidad al levantarse; si no, el jugador puede hacer
  *infinite* sobre un enemigo caído.

```go
func (e *Entity) UpdateHurt() {
	if e.State != StateHurt {
		return
	}
	if e.Hurt > 0 {
		e.Hurt--
	}
	e.Vel.X *= 0.85
	e.Vel.Y *= 0.85
	if e.Hurt == 0 {
		e.State = StateIdle
		e.Anim.Play("idle", true)
	}
}
```

La fricción debe cortar a cero:

```go
if math.Abs(e.Vel.X) < 0.05 {
	e.Vel.X = 0
}
```

El bloqueo necesita su propio temporizador:

```go
func (e *Entity) UpdateBlock() {
	if e.State != StateBlock {
		return
	}
	if e.Blockstun > 0 {
		e.Blockstun--
	}
	e.Vel.X *= 0.85
	if e.Blockstun == 0 {
		e.State = StateIdle
		e.Anim.Play("idle", true)
	}
}
```

Si el jugador mantiene «atrás», vuelve a entrar en `StateBlock` en el siguiente
tick; si no, queda en `Idle`.

### Invulnerabilidad

```go
func (e *Entity) UpdateInvuln() {
	if e.Invuln > 0 {
		e.Invuln--
	}
}
```

Usos típicos:

- Al levantarse (`GetUp`): 20–30 ticks.
- Al empezar una súper: durante el startup.
- Al recibir un golpe concreto: no des invulnerabilidad a todo, o romperás los
  combos.

La invulnerabilidad se comprueba antes de aplicar daño, no en la detección. Así
un ataque puede «whiffear» contra alguien invulnerable y no marcarlo como
golpeado; decide si eso es lo que quieres.

## 7. Bloqueo

```go
func (e *Entity) CanBlock(attacker *Entity) bool {
	if e.State == StateAttack || e.State == StateHurt || e.State == StateDown {
		return false
	}
	if !e.Blocking {
		return false
	}
	// Solo bloquea si mira hacia el atacante.
	dir := 1.0
	if attacker.Pos.X < e.Pos.X {
		dir = -1
	}
	return float64(e.Facing) == dir
}
```

Detalles de diseño:

- El bloqueo se activa manteniendo «atrás» (dirección opuesta a la mirada), no un
  botón dedicado, en el estilo clásico.
- `Blockstun` impide actuar; `ChipDamage` castiga el bloqueo si quieres evitar
  turtling.
- Un ataque `Unblockable` (agarre, súper) ignora el bloqueo.
- El bloqueo no debe funcionar por la espalda; si el atacante está detrás, el
  jugador recibe daño.
- Si el jugador bloquea en el aire, decide si puede o no; no lo dejes al azar.

## 8. Combos, buffer y cancelaciones

### Buffer

```go
type InputBuffer struct {
	attackAt int64
	window   int64
}

func (b *InputBuffer) PressAttack(now int64) {
	b.attackAt = now
}

func (b *InputBuffer) ConsumeAttack(now int64) bool {
	if now-b.attackAt > b.window {
		return false
	}
	b.attackAt = -1 << 60
	return true
}
```

Ventana típica: 3–6 ticks. Si es muy grande, el personaje ataca «solo» mucho
después de pulsar; si es muy pequeña, el juego se siente rígido.

### Cancelación

Solo permites cancelar si el ataque anterior **ha conectado** y estás en la
ventana:

```go
func (e *Entity) TryAttack(next *Move) bool {
	if e.State == StateAttack && !e.CanCancel() {
		return false
	}
	if e.State == StateAttack && !contains(e.Move.Next, next.Name) {
		return false
	}
	e.StartAttack(next)
	return true
}
```

Esto crea el combo `jab → jab2 → finisher`. Si quieres permitir cancelar al
bloquear, marca también ese caso en `CanCancel` (por ejemplo, con un campo
`MoveBlocked` que pongas a `true` en la rama de bloqueo).

### Contador de combo

```go
func (w *World) UpdateCombo() {
	for _, e := range w.Entities {
		if e.ComboTimer > 0 {
			e.ComboTimer--
			if e.ComboTimer == 0 {
				e.ComboCount = 0
			}
		}
	}
}
```

El HUD muestra `ComboCount` cuando es mayor que 1. Un temporizador de 30–60 ticks
funciona bien.

### Escalado de daño

Para evitar combos infinitos:

```go
scale := []float64{1.0, 0.9, 0.8, 0.7, 0.6}
i := min(a.ComboCount-1, len(scale)-1)
damage := int(float64(m.Damage) * scale[i])
```

No es obligatorio en el primer prototipo, pero es una herramienta de balance.

### Juggle

Si permites golpear en el aire, limita los golpes:

```go
if t.Pos.Z > 0 {
	t.JuggleCount++
	if t.JuggleCount > m.JuggleLimit {
		t.State = StateDown
		t.Vel.Z = 0
	}
}
```

Sin límite, un jefe puede quedarse flotando para siempre.

## 9. IA de enemigos

La IA de un beat 'em up no necesita ser brillante; necesita ser **legible y
justa**. Un enemigo que ataca sin telegraph es frustrante; diez enemigos atacando
a la vez es imposible.

### Máquina simple

```go
type AIState int

const (
	AISpawn AIState = iota
	AIApproach
	AIWait
	AIAttack
	AIRetreat
	AIStunned
)

type AI struct {
	State    AIState
	Cooldown int
	Token    bool
	Aggro    float64 // 0..1, personalidad
}
```

Cada tick:

```text
1. Si está en hitstun/hitstop, no decide.
2. Calcula dx, dy respecto al jugador.
3. Si no tiene token de ataque, se acerca y espera.
4. Si tiene token y está en rango, ataca.
5. Tras atacar, suelta el token y espera un cooldown.
6. Si está muy cerca pero en otra profundidad, se reposiciona.
```

### Tokens de ataque

```go
type World struct {
	AttackTokens int // por ejemplo, 2
}

func (w *World) RequestToken() bool {
	if w.AttackTokens <= 0 {
		return false
	}
	w.AttackTokens--
	return true
}

func (w *World) ReleaseToken() {
	w.AttackTokens++
}
```

Así solo dos enemigos atacan a la vez; el resto rodea y espera. Es la diferencia
entre un beat 'em up y una turba aleatoria.

### Telegraph

Antes de atacar, el enemigo debe mostrar intención:

- Se detiene y mira al jugador.
- Cambia de animación o parpadea.
- Espera `startup` ticks (que ya son parte del ataque).
- El ataque tiene un alcance claro.

Si el `startup` es de 4 ticks, el jugador apenas puede reaccionar. Para enemigos
peligrosos, usa 12–20 ticks y una animación distinta.

### Personalidad

```go
type EnemyDef struct {
	Health      int
	Speed       float64
	Aggression  float64 // probabilidad de pedir token
	AttackRange float64
	DepthRange  float64
	Moves       []string
	Cooldown    int
}
```

Con tres arquetipos (rápido y débil, lento y fuerte, intermedio) ya hay variedad
suficiente para el primer nivel.

### Errores de IA

- Todos los enemigos comparten el mismo `Cooldown` y atacan sincronizados.
- El enemigo se queda pegado al jugador sin atacar nunca.
- El enemigo ataca desde fuera de alcance y el golpe no conecta.
- El enemigo cambia de objetivo cada tick y parece indeciso.
- La IA reacciona a la posición del jugador en el mismo tick en que este se mueve;
  añade un pequeño retardo o usa la posición del tick anterior.

## 10. Balance y «game feel»

Checklist de sensaciones:

- El golpe ligero se siente rápido; el pesado, contundente.
- El jugador puede cancelar tras conectar, pero no al fallar.
- El hitstop dura lo suficiente para leerse (3–8 ticks).
- El knockback aleja al enemigo sin sacarlo de la pantalla.
- El enemigo muestra dolor y no sigue atacando como si nada.
- La cámara no se sacude en cada golpe; reserva la sacudida para impactos fuertes.
- El sonido y la chispa coinciden con el tick activo.
- El jugador tiene una forma de escapar de la esquina (salto, ataque, esquiva).

Mide el tiempo para matar (TTK):

```text
TTK = vida_enemigo / daño_por_combo * duración_del_combo
```

Si un enemigo básico tarda 20 segundos, el nivel se hace eterno. Si tarda 2, no
hay tensión. Ajusta vida y daño antes de añadir más enemigos.

## 11. Errores frecuentes

| Síntoma | Causa probable |
|---|---|
| Un ataque golpea dos veces | `HitTargets` se reinicia por frame |
| El golpe no conecta nunca | Hitbox activa un tick antes/después |
| El enemigo queda congelado | Hitstop no se decrementa |
| El combo es infinito | No hay límite de juggle ni escalado |
| Se puede bloquear por la espalda | `CanBlock` no comprueba la dirección |
| El atacante cancela sin conectar | `CanCancel` no exige `MoveHit` |
| Todos los enemigos atacan a la vez | Falta token system |
| El jugador no puede reaccionar | Startup demasiado corto o sin telegraph |
| El daño depende del orden | Se resuelve dentro del recorrido |

## 12. Ejercicio del capítulo

1. Implementa `Move`, `IsActive` y la tabla de tres ataques.
2. Implementa `StartAttack`, `AdvanceAttack` y la transición a `Idle`.
3. Recoge impactos y resuelve daño una sola vez por objetivo y ataque.
4. Añade hitstop, hitstun y knockback; comprueba que se sienten distintos en un
   golpe ligero y uno pesado.
5. Añade bloqueo con `Blockstun` y daño de chip.
6. Implementa el buffer de ataque y el combo `jab → jab2 → finisher`.
7. Crea un enemigo con `Approach`, `Wait` y `Attack`, y un token global de 2.
8. Escribe tests:
   - un ataque activo golpea una sola vez;
   - un ataque en recovery no golpea;
   - bloquear de frente reduce el daño;
   - el hitstop se decrementa y no se acumula sin límite.
9. Ajusta la tabla hasta que un enemigo básico muera en 3–5 combos.

Criterio de aceptación: puedes explicar, tick a tick, qué ocurre desde que pulsas
el botón hasta que el enemigo cae, y el debug muestra exactamente esos ticks.

## 13. Referencias

- `inpututil` para bordes de teclado/gamepad.
- `examples/platformer` para gravedad y estados simples.
- `vector` para dibujar hitboxes.
- `audio` (capítulo 07) para el feedback sonoro.
- La lectura de frame data de juegos del género es una buena referencia de diseño;
  no copies números, copia la estructura.
