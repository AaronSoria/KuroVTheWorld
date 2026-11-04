# Guía para construir un beat 'em up con Ebitengine

Esta guía está pensada para que puedas diseñar e implementar tu propio beat 'em up
sin depender de una IA que te escriba el código. Explica el motor, la arquitectura,
las decisiones típicas del género y los errores que más tiempo hacen perder. No es
una transcripción de la documentación oficial ni un proyecto terminado: es una hoja
de ruta con conceptos, bocetos y ejercicios para que escribas cada pieza tú mismo.

> **Versión de Ebitengine**: este proyecto usa Ebitengine v2.9.9 y Go 1.26
> (consulta `go.mod`). La API de v2 es estable, pero verifica siempre las firmas
> con `go doc` contra la versión que tengas instalada. Al final de cada capítulo
> hay referencias al código fuente del motor y a los ejemplos oficiales.

---

## 1. Para quién es esta guía

- Sabes programar en Go a nivel básico/intermedio: estructuras, slices, punteros,
  interfaces y tests.
- Conoces (o quieres aprender) Ebitengine. No hace falta experiencia previa en
  gráficos.
- Quieres hacer un beat 'em up 2D con scroll lateral, varios enemigos, combos y
  jefes. El género también se llama *belt scroller*, *side-scrolling brawler* o
  *hack and slash* 2D.

No cubre modelado 3D, multijugador en red, animación esquelética avanzada ni
editores de niveles completos. Esas piezas se pueden añadir después, cuando el
núcleo funcione.

## 2. El objetivo: un «vertical slice», no un juego enorme

El error más común del primer proyecto es intentar hacer *Streets of Rage 4*.
Empieza con una rebanada jugable de 10–15 minutos:

| Pieza | Alcance mínimo |
|---|---|
| Jugador | Caminar en 2 ejes, saltar, 3 ataques encadenables, recibir daño |
| Enemigos | 3 tipos con comportamientos distintos y un jefe |
| Nivel | 1 escenario con scroll, 3 oleadas y una puerta/bloqueo |
| Combate | Hitboxes, hitstun, knockback, hitstop, bloqueo e invulnerabilidad |
| Presentación | Sprites animados, HUD, música y efectos de sonido |
| Flujo | Título → juego → pausa → game over → reintentar |
| Persistencia | Ajustes (volumen, teclas) y progreso básico |

Cuando esa rebanada se sienta bien, ampliar contenido es mucho más barato que
rehacer sistemas.

## 3. Cómo usar la guía

Lee los capítulos en orden la primera vez. Cada uno tiene:

1. **Conceptos**: el modelo mental antes de tocar código.
2. **Bocetos de código**: fragmentos pequeños, no un juego completo. Escríbelos a
   mano y adáptalos; no los pegues sin entenderlos.
3. **Ejercicio**: algo verificable que debes terminar antes de avanzar.
4. **Errores frecuentes**: síntomas y causas.
5. **Referencias**: dónde mirar cuando la guía se quede corta.

Una regla práctica: si no puedes explicar una estructura de datos o un orden de
actualización sin mirar el código, todavía no la domines. Vuelve al ejercicio.

## 4. Ruta recomendada

| Etapa | Capítulo | Resultado |
|---|---|---|
| 0 | [01 — Ebitengine esencial](01-ebitengine-esencial.md) | Ventana, bucle, dibujo, input |
| 1 | [02 — Arquitectura](02-arquitectura.md) | Escenas, entidades, orden de update |
| 2 | [03 — Sprites y animación](03-sprites-animacion.md) | Personaje animado y ordenado por profundidad |
| 3 | [04 — Espacio, movimiento y colisiones](04-espacio-movimiento-colisiones.md) | Plano 2.5D, cajas, empuje |
| 4 | [05 — Combate](05-combate.md) | Frame data, golpes, hitstop, IA |
| 5 | [06 — Cámara, niveles y oleadas](06-camara-niveles-oleadas.md) | Scroll, bloqueos, spawns |
| 6 | [07 — Audio](07-audio.md) | Música, efectos, mezcla y latencia |
| 7 | [08 — HUD, texto y menús](08-ui-menus.md) | HUD, pausa, opciones, accesibilidad |
| 8 | [09 — Guardado y progreso](09-guardado.md) | Ajustes, saves, migraciones |
| 9 | [10 — Rendimiento, pruebas y publicación](10-rendimiento-pruebas-publicacion.md) | 60 fps estables, tests, build |
| — | [Apéndice: datos, API y glosario](apendice-datos-api-glosario.md) | Chuleta y esquemas JSON |

## 5. Convenciones de esta guía

- **Unidades**: píxeles lógicos del mundo. El tamaño de la ventana no cambia las
  reglas del juego; cambia solo la escala de presentación.
- **Eje Y**: hacia abajo, como en las imágenes. En el mundo del beat 'em up, `Y`
  representa la profundidad del suelo (más abajo = más cerca de la cámara) y `Z`
  la altura (saltos).
- **Tiempo**: la lógica corre a *ticks* fijos. A 60 TPS, un tick = 1/60 s. Los
  frames de ataque, hitstun e hitstop se cuentan en ticks, no en segundos.
- **Código**: identificadores en inglés, comentarios y documentación en español.
  Es lo habitual en proyectos de Go y facilita comparar con los ejemplos.
- **Sin dependencias innecesarias**: `math`, `image`, `encoding/json`, `embed` y
  la biblioteca estándar bastan para casi todo. Añade librerías cuando midas una
  necesidad concreta.

## 6. Requisitos y primer arranque

Instala una versión reciente de Go (consulta el `go.mod` de Ebitengine; este repo
usa 1.26) y comprueba:

```bash
go version
```

Crea el módulo y añade Ebitengine:

```bash
mkdir mi-beatemup && cd mi-beatemup
go mod init ejemplo.com/mi-beatemup
go get github.com/hajimehoshi/ebiten/v2
```

Un `main.go` mínimo para verificar que todo funciona:

```go
package main

import (
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

type Game struct{}

func (g *Game) Update() error { return nil }

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0x18, 0x18, 0x20, 0xff})
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return 480, 270
}

func main() {
	ebiten.SetWindowSize(960, 540)
	ebiten.SetWindowTitle("Mi beat 'em up")
	if err := ebiten.RunGame(&Game{}); err != nil {
		log.Fatal(err)
	}
}
```

Comprueba que aparece la ventana. En Linux necesitas las dependencias de sistema
habituales de Ebitengine (X11, OpenGL, ALSA). La página oficial de instalación
las lista por distribución.

## 7. Filosofía de trabajo

1. **Haz que se mueva antes de que se vea bien.** Un rectángulo que camina y
   golpea enseña más que un sprite bonito que no responde.
2. **Una variable, un dueño.** La posición la modifica el sistema de movimiento;
   la vida, el sistema de combate; el input, la capa de input. Evita que todo
   escriba todo.
3. **Depura con dibujo, no con `fmt.Println` dentro del bucle.** Cajas de color,
   estados y contadores en pantalla ahorran horas.
4. **Fija el comportamiento con tests.** La matemática de colisiones, el frame
   data y las transiciones de estado se prueban sin abrir una ventana.
5. **Ajusta con datos, no recompilando.** Velocidades, daño y frames en un struct
   o JSON te dejan iterar rápido.
6. **El «game feel» es parte del diseño, no un adorno final.** Hitstop, sacudida,
   sonido y retroceso se diseñan desde el principio.

## 8. Referencias oficiales

- Portal de Ebitengine: <https://ebitengine.org/>
- Documentación del paquete: <https://pkg.go.dev/github.com/hajimehoshi/ebiten/v2>
- Ejemplos del repositorio: <https://github.com/hajimehoshi/ebiten/tree/main/examples>
- Consejos de rendimiento: <https://ebitengine.org/en/documents/performancetips.html>
- Código fuente de Ebitengine (en un checkout del motor): `ebiten.go`, `image.go`,
  `geom.go`, `input.go`, `audio/`, `text/v2/`, `vector/`, `inpututil/`.
- Ejemplos del repositorio de Ebitengine útiles para esta guía:
  `examples/platformer` (movimiento y cámara simple), `examples/animation`
  (sprite sheet), `examples/camera` (transformación de cámara), `examples/2048`
  (arquitectura y escenas), `examples/audio` (reproductores), `examples/text`
  (fuentes).

Cuando algo no esté claro, `go doc` es más fiable que cualquier tutorial:

```bash
go doc github.com/hajimehoshi/ebiten/v2.DrawImageOptions
go doc github.com/hajimehoshi/ebiten/v2/inpututil
go doc github.com/hajimehoshi/ebiten/v2/audio.Player
```

---

Empieza por [01 — Ebitengine esencial](01-ebitengine-esencial.md).
