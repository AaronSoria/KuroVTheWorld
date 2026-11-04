# 10 — Rendimiento, pruebas y publicación

Un beat 'em up a 60 TPS con pocos personajes no debería ser un problema de
rendimiento. Cuando algo va mal, casi siempre es por asignaciones por frame, por
leer píxeles de la GPU o por dibujar de forma que rompe el batching, no por el
número de sprites.

## 1. El presupuesto de frame

A 60 TPS y 60 FPS tienes unos 16,6 ms por frame para `Update` + `Draw` + trabajo
del motor. En la práctica, apunta a menos de 8 ms de tu código para tener margen.

```go
ebitenutil.DebugPrint(screen, fmt.Sprintf(
	"TPS %.1f  FPS %.1f",
	ebiten.ActualTPS(), ebiten.ActualFPS(),
))
```

Interpretación:

- `ActualTPS` bajo y `ActualFPS` alto: `Update` va lento o el juego está pausado.
- `ActualFPS` bajo: el coste está en `Draw` o en la presentación.
- Ambos bajos: puede ser el sistema, VSync o una carga externa.

Mide antes de optimizar. Un cambio «obvio» sin medición suele complicar el código
sin mejorar nada.

## 2. Asignaciones y basura

En `Update` y `Draw` evita:

- `fmt.Sprintf` para textos que no cambian.
- Crear slices, mapas o imágenes por frame.
- Convertir `string` a `[]byte` o viceversa en bucles.
- `time.Now()`.
- Cierres (closures) que capturen y asignen en bucles calientes.
- `append` sin capacidad reservada en listas que crecen cada tick.

```go
// Mal: asigna en cada frame.
msg := fmt.Sprintf("x=%f y=%f", e.Pos.X, e.Pos.Y)

// Mejor: solo recalcula cuando cambia el valor mostrado.
if int(e.Pos.X) != g.lastShownX {
	g.lastShownX = int(e.Pos.X)
	g.posText = strconv.Itoa(g.lastShownX)
}
```

Para medir asignaciones:

```bash
go test -bench=. -benchmem ./...
```

Reutiliza buffers:

```go
type World struct {
	hitScratch []Hit
}

func (w *World) CollectHits() []Hit {
	w.hitScratch = w.hitScratch[:0]
	// ... append ...
	return w.hitScratch
}
```

Cuidado: si devuelves el slice y lo guardas, el siguiente tick lo sobrescribe.
Úsalo solo dentro del tick o copia cuando salga del ámbito.

## 3. Dibujo y batching

Ebitengine agrupa comandos de dibujo compatibles. Para aprovecharlo:

- Dibuja en orden de destino: no alternes entre la pantalla y un offscreen sin
  necesidad.
- Mantén el mismo blend, filtro y shader en draws consecutivos.
- Reutiliza imágenes; no crees una imagen por sprite.
- Deja que el atlas automático haga su trabajo; no marques `Unmanaged` sin una
  razón medida.
- No cambies uniforms de un shader entre objetos si puedes evitarlo; puede
  separar comandos.
- No uses `DrawTriangles` manualmente solo porque hay muchos sprites. Primero
  comprueba si ya se agrupan.

Para inspeccionar los comandos internos:

```bash
go run -tags=ebitenginedebug ./cmd/game
```

La salida describe los comandos del motor, no necesariamente cada draw call del
backend. Compara escenas representativas antes y después de un cambio.

### Offscreen: solo cuando aporta

Un offscreen sirve para:

- Oscurecer el mundo tras un menú.
- Aplicar un efecto de pantalla completa.
- Renderizar el mundo a menor resolución y escalarlo.

No sirve para «optimizar» por sí solo: añade una pasada de render y puede sacar
las imágenes del atlas de fuentes. Si lo usas, reutilízalo y redimensiónalo solo
cuando cambie el tamaño.

### Lectura de píxeles

`Image.At`, `ReadPixels` y similares pueden forzar la sincronización de la GPU.
Nunca los uses para comprobar colisiones ni para lógica por píxel. Si necesitas
saber el color de un píxel, calcula la información en CPU o hazlo una vez al
cargar.

## 4. Pixel art nítido

- `Layout` fijo (por ejemplo 480×270) y ventana a múltiplos enteros.
- `FilterNearest` (valor por defecto).
- Redondea posiciones de cámara y sprites al dibujar.
- Evita escalas no enteras en el mundo; si necesitas zoom, renderiza a otra
  resolución lógica.
- No uses mipmaps para pixel art; el filtro lineal y los mipmaps suavizan.
- Cuida el *bleeding* entre frames de un atlas: deja padding o usa nearest.

Si quieres profundizar, el repositorio incluye la skill
`skills/avoiding-blurry-ebitengine-rendering/SKILL.md`.

## 5. Pruebas: qué testear y qué no

Testea la lógica que puede romperse sin que se vea:

- Matemática de colisiones (`AABB`, `Overlaps`, `clamp`).
- Frame data (`IsActive`, `Total`, cancelaciones).
- Resolución de golpes (una vez por objetivo, bloqueo direccional).
- Hitstop y temporizadores.
- Máquina de estados (transiciones válidas).
- Oleadas y triggers.
- Guardado y migraciones.
- Parseo y validación de niveles.

No necesitas testear que Ebitengine dibuja un píxel; eso ya lo hace el motor.

### Separar lógica de Ebitengine

Si `World.Update` recibe un `InputState`, puedes probarlo sin ventana:

```go
func TestAttackHitsOnce(t *testing.T) {
	w := NewTestWorld()
	player := w.AddPlayer(100, 200)
	enemy := w.AddEnemy(120, 200)

	player.StartAttack(Moves["jab"])
	for i := 0; i < Moves["jab"].Total(); i++ {
		w.Step(InputState{})
	}

	if got := enemy.Health; got != enemy.MaxHealth-Moves["jab"].Damage {
		t.Fatalf("vida = %d", got)
	}
	if got := player.ComboCount; got != 1 {
		t.Fatalf("combos = %d", got)
	}
}
```

Tabla de tests útiles:

| Test | Qué comprueba |
|---|---|
| `TestAttackActiveWindow` | Solo golpea en `[Startup, Startup+Active)` |
| `TestHitOncePerAttack` | Dos ticks activos no duplican daño |
| `TestBlockDirection` | Bloquear de frente funciona; por la espalda no |
| `TestHitstop` | Se decrementa y no se acumula sin tope |
| `TestWaveTransitions` | `Waiting → Spawning → Fighting → Cleared` |
| `TestSaveRoundTrip` | Escribir y leer conserva los datos |
| `TestSaveCorrupt` | JSON roto no rompe el arranque |
| `TestLevelValidate` | Spawns fuera de bounds fallan al cargar |

### Tests con ventana

La mayoría de la lógica no la necesita. Si un test requiere graphics o
`internal/testing`, en Linux ejecútalo bajo Xvfb para no abrir ventanas en tu
escritorio:

```bash
xvfb-run -a go test ./...
```

Regla del repositorio: no uses un `go test` normal para tests gráficos en Linux.
Para lógica pura, `go test ./...` es suficiente.

### Determinismo

- Usa `math/rand/v2` con una semilla fija en tests y en repeticiones.
- No dependas del orden de un `map` para decisiones de juego.
- Ordena por `ID` antes de resolver impactos.
- No uses `time.Now()` en la simulación.
- Guarda la semilla y, si puedes, un log de inputs para reproducir bugs.

## 6. Depuración visual

Un overlay bien hecho ahorra horas:

```text
F1: cajas     F2: estados     F3: cámara     F4: rendimiento
```

Muestra:

- Cajas de cuerpo, hurtbox y hitbox activa.
- Estado, `StateTick`, `MoveTick`, `Hurt`, `Invuln`, `Hitstop`.
- Vida, combo y objetivo de la oleada.
- Cámara, bounds y triggers.
- Número de entidades y de comandos si usas `ebitenginedebug`.

Para entender un combo, baja temporalmente los TPS:

```go
if debugSlow {
	ebiten.SetTPS(10)
} else {
	ebiten.SetTPS(60)
}
```

Así ves los ticks uno a uno. No dejes el TPS bajo en producción.

## 7. Perfilado

Para CPU, usa `runtime/pprof` como en `examples/blocks`:

```go
var cpuProfile = flag.String("cpuprofile", "", "write cpu profile to file")

func main() {
	flag.Parse()
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}
	// RunGame...
}
```

```bash
go run ./cmd/game -cpuprofile=cpu.prof
go tool pprof -http=:8080 cpu.prof
```

Busca:

- Funciones que se llaman más de lo esperado.
- Asignaciones dentro de `Update`/`Draw`.
- `fmt`, `reflect`, `json` en rutas calientes.
- Bucles `O(n²)` con muchas entidades.

Para memoria, `go test -benchmem` suele ser más útil que un perfil completo.

## 8. Publicación

### Antes de publicar

```bash
gofmt -w .
go vet ./...
go test ./...
go build ./...
```

- Elimina el debug o actívalo solo con una tecla.
- Comprueba que el juego arranca sin assets externos si usas `embed`.
- Prueba en una máquina sin Go instalado.
- Comprueba los licencias de fuentes, música y sprites.
- Añade una pantalla de créditos y un aviso de licencia.
- Versiona el save y prueba una actualización desde la versión anterior.

### Escritorio

```bash
go build -o bin/mi-beatemup ./cmd/game
```

Para Windows/macOS, lo más fiable es compilar en el sistema destino o usar la
integración de CI adecuada; las dependencias gráficas y de audio pueden variar.
No prometas binarios universales sin probarlos.

### Web (WASM)

```bash
GOOS=js GOARCH=wasm go build -o web/game.wasm ./cmd/game
# Go 1.24+:
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/
# Go anterior a 1.24:
# cp "$(go env GOROOT)/misc/wasm/wasm_exec.js" web/
```

Necesitas un `index.html` que cargue `wasm_exec.js` y arranque `game.wasm`, y
servir por HTTP (no abras el archivo con `file://`). Comprueba:

- Tamaño del WASM.
- Audio tras la primera interacción.
- Teclado y gamepad en el navegador.
- Pantalla completa y escalado.
- Guardado: `localStorage` o descarga de archivo; el sistema de archivos del
  navegador es distinto. Si usas `os.UserConfigDir`, no funcionará igual.

La documentación oficial de Ebitengine tiene una guía específica de WebAssembly;
síguela para la versión de Go que uses.

## 9. Hoja de ruta del vertical slice

| Hito | Entregable | Criterio de aceptación |
|---|---|---|
| 0 | Ventana + caja que camina | 60 TPS estables, input correcto |
| 1 | Escenas título/juego/pausa | Transiciones sin fugas de estado |
| 2 | Sprite animado y ordenado | Pies anclados, sin saltos al girar |
| 3 | Plano 2.5D y colisiones | El jugador se desliza por paredes |
| 4 | Un ataque con hitbox | Golpea una vez, se ve el frame activo |
| 5 | Hitstop, hitstun, bloqueo | El combate se lee y se siente |
| 6 | Combo de tres golpes | Buffer y cancelación funcionan |
| 7 | Un enemigo con IA | Se acerca, telegrafía y ataca |
| 8 | Oleadas y cámara | Tres oleadas, cámara bloqueada y liberada |
| 9 | HUD, menús, audio | Pausa, volumen y combo visibles |
| 10 | Guardado y nivel completo | Reabrir conserva ajustes y progreso |
| 11 | Pulido | Sin bugs bloqueantes, 10–15 min jugables |

### Recortes de alcance recomendados

Para el primer vertical slice, **no** hagas:

- Cooperativo en red o local (multiplica la complejidad de cámara y estados).
- Editor de niveles.
- Animación esquelética.
- Sistema de inventario o armas con durabilidad.
- Cinemáticas largas.
- Más de un nivel.
- Más de tres tipos de enemigo y un jefe.
- Multijugador, ranking online, logros.

Añade cada uno cuando el núcleo esté terminado y probado.

## 10. Playtesting

1. Deja jugar a alguien sin explicarle los controles. Anota qué intenta y qué no
   entiende.
2. No le ayudes durante los primeros 5 minutos.
3. Pregunta: «¿Qué crees que hace este enemigo?», «¿Por qué crees que te han
   golpeado?», «¿Qué harías ahora?».
4. Mide: muertes, tiempo por oleada, ataques usados, veces que se queda sin vida.
5. Cambia una cosa a la vez (vida, daño, startup, velocidad).

Señales de alarma:

- El jugador no sabe que puede bloquear.
- Los enemigos atacan desde fuera de la pantalla.
- El jugador se queda atrapado en una esquina sin opciones.
- Los combos no salen nunca o salen solos.
- El jugador no entiende por qué ha muerto.

## 11. Checklist final

- [ ] 60 TPS estables en el hardware objetivo.
- [ ] Sin asignaciones evidentes en `Update`/`Draw` (medido).
- [ ] Pixel art nítido a la escala de ventana elegida.
- [ ] Input de teclado y gamepad, con buffer.
- [ ] Pausa real, menús navegables y opciones aplicadas.
- [ ] Guardado atómico y migración probada.
- [ ] Combate legible: hitstop, hitstun, bloqueo, invulnerabilidad.
- [ ] IA justa: tokens, telegraph, cooldowns.
- [ ] Oleadas y cámara sin bloqueos.
- [ ] Audio sin solapamientos ni latencia excesiva.
- [ ] Tests de lógica y de guardado en verde.
- [ ] Créditos y licencias de assets.
- [ ] Build de escritorio (y web si aplica) probado fuera del entorno de desarrollo.

## 12. Referencias

- `skills/efficient-ebitengine-rendering/SKILL.md`.
- `skills/avoiding-blurry-ebitengine-rendering/SKILL.md`.
- `examples/blocks` para perfilado con `pprof`.
- `examples/2048` para tests de lógica y estado.
- Documentación de rendimiento: <https://ebitengine.org/en/documents/performancetips.html>
- Guía de WebAssembly en la documentación oficial de Ebitengine.
