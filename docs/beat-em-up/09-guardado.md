# 09 — Guardado, ajustes y progreso

Un beat 'em up se juega en sesiones cortas. Si el jugador pierde el volumen que
configuró o el nivel que desbloqueó, la experiencia se rompe. El guardado debe ser
aburrido, pequeño y difícil de corromper.

## 1. Qué guardar y qué no

Guarda **datos**, nunca recursos ni estado temporal:

| Guardar | No guardar |
|---|---|
| Volumen master/música/SFX | `*ebiten.Image` |
| Teclas asignadas | Punteros a entidades |
| Niveles desbloqueados | Posición exacta de un enemigo |
| Mejor puntuación | Estado de una animación |
| Dificultad elegida | `time.Time` de la sesión |
| Idioma | Rutas absolutas del sistema |

Los assets se identifican por nombre (`"stage1"`, `"thug"`) y se cargan de nuevo
al arrancar. Si guardas rutas absolutas, el save deja de funcionar en otra
máquina.

## 2. Dónde guardar

Usa el directorio de configuración del usuario:

```go
func saveDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "mi-beatemup"), nil
}
```

En Linux suele ser `~/.config/mi-beatemup`; en Windows, `%AppData%`; en macOS,
`~/Library/Application Support`. No escribas junto al ejecutable: puede estar en
una carpeta de solo lectura o dentro de un paquete de solo lectura.

Si `os.UserConfigDir` falla, decide un fallback explícito (por ejemplo, el
directorio actual) y avisa en el log. No ignores el error y escribas en cualquier
sitio.

## 3. Estructura del archivo

```go
const saveVersion = 2

type SaveData struct {
	Version  int      `json:"version"`
	Settings Settings `json:"settings"`
	Progress Progress `json:"progress"`
}

type Settings struct {
	MasterVolume float64          `json:"masterVolume"`
	MusicVolume  float64          `json:"musicVolume"`
	SFXVolume    float64          `json:"sfxVolume"`
	Fullscreen   bool             `json:"fullscreen"`
	Keybinds     map[string]int   `json:"keybinds"` // acción -> ebiten.Key
	Language     string           `json:"language"`
	Shake        bool             `json:"shake"`
	Flashes      bool             `json:"flashes"`
}

type Progress struct {
	UnlockedLevel int            `json:"unlockedLevel"`
	BestScores    map[string]int `json:"bestScores"`
	TotalKills    int            `json:"totalKills"`
	Cleared       bool           `json:"cleared"`
}
```

Ventajas:

- `Version` permite migrar sin adivinar.
- Los nombres JSON son estables aunque renombres campos de Go.
- `omitempty` puede ayudar con campos opcionales, pero no lo uses para valores
  donde `0` es significativo.
- Un mapa de puntuaciones por nivel es más flexible que un solo entero.

### Valores por defecto

```go
func DefaultSave() *SaveData {
	return &SaveData{
		Version: saveVersion,
		Settings: Settings{
			MasterVolume: 0.8,
			MusicVolume:  0.6,
			SFXVolume:    0.9,
			Shake:        true,
			Flashes:      true,
			Language:     "es",
			Keybinds: map[string]int{
				"attack": int(ebiten.KeyJ),
				"heavy":  int(ebiten.KeyK),
				"jump":   int(ebiten.KeySpace),
				"pause":  int(ebiten.KeyEscape),
			},
		},
		Progress: Progress{
			UnlockedLevel: 1,
			BestScores:    map[string]int{},
		},
	}
}
```

Ten defaults sensatos: si el archivo no existe, el juego debe arrancar igual.

## 4. Cargar sin romper el juego

```go
func Load() (*SaveData, error) {
	path, err := savePath()
	if err != nil {
		return DefaultSave(), err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return DefaultSave(), nil
	}
	if err != nil {
		return DefaultSave(), err
	}

	var s SaveData
	if err := json.Unmarshal(data, &s); err != nil {
		// No borres el archivo del usuario: guárdalo como .bak.
		_ = os.Rename(path, path+".bak")
		return DefaultSave(), fmt.Errorf("save corrupto: %w", err)
	}

	s.migrate()
	s.clamp()
	return &s, nil
}
```

Reglas:

- Archivo inexistente no es un error: usa defaults.
- Archivo corrupto no debe impedir jugar: haz copia y usa defaults.
- Después de cargar, valida rangos (`volume` en `[0,1]`, nivel existente,
  teclas válidas). Un JSON manipulado no debe romper el juego.
- No uses `panic` para datos de usuario.

### Migraciones

```go
func (s *SaveData) migrate() {
	if s.Version >= saveVersion {
		return
	}
	if s.Version < 1 {
		// Versión 0 no tenía Settings; crea los valores por defecto.
		s.Settings = DefaultSave().Settings
		s.Version = 1
	}
	if s.Version < 2 {
		// Versión 1 no tenía BestScores.
		if s.Progress.BestScores == nil {
			s.Progress.BestScores = map[string]int{}
		}
		s.Version = 2
	}
}
```

Regla de oro: una migración solo avanza de versión y nunca pierde datos que el
usuario pueda necesitar. Si no puedes migrar, conserva el archivo y empieza de
cero, pero no lo sobrescribas sin copia.

## 5. Guardar de forma atómica

Escribe en un archivo temporal y renombra. Así un corte de luz no deja un JSON a
medias:

```go
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".save-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // si algo falla antes del rename

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
```

```go
func Save(s *SaveData) error {
	path, err := savePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, data)
}
```

Detalles:

- El temporal debe estar en el **mismo directorio** que el destino; si está en
  `/tmp` y el destino en otro sistema de archivos, el rename puede fallar.
- `Sync` reduce el riesgo de pérdida, aunque no es gratis. Guarda con poca
  frecuencia.
- En algunos sistemas `os.Rename` no es atómico; aun así es mejor que escribir
  directamente sobre el archivo bueno.
- No guardes en cada tick. Marca `dirty` y escribe al cambiar de escena, al
  terminar un nivel o cada pocos segundos.

## 6. Cuándo guardar

Momentos razonables:

- Al cambiar cualquier ajuste (con un pequeño debounce).
- Al terminar una oleada o un nivel.
- Al desbloquear algo.
- Al salir al título.
- Al cerrar la ventana: `RunGame` vuelve cuando se cierra, así que puedes guardar
  después de `RunGame` si el bucle terminó limpiamente. No dependas de ello para
  el progreso importante: guarda también en los hitos.

```go
if err := ebiten.RunGame(g); err != nil {
	log.Fatal(err)
}
if err := game.SaveIfDirty(); err != nil {
	log.Printf("no se pudo guardar: %v", err)
}
```

No bloquees el hilo de dibujo con un guardado grande. Para un JSON de pocos KB no
es un problema; si crece, escribe en una goroutine con una cola y protege los
datos con un mutex o copia inmutable.

## 7. Aplicar ajustes al cargar

Cargar no basta; hay que aplicar:

```go
func ApplySettings(s Settings, audio *AudioSystem) {
	audio.SetMaster(s.MasterVolume)
	audio.SetMusic(s.MusicVolume)
	audio.SetSFX(s.SFXVolume)
	ebiten.SetFullscreen(s.Fullscreen)
	// Keybinds: reconstruye el mapa de acciones.
}
```

Valida antes de aplicar:

```go
func clamp01(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
```

Un volumen `NaN` puede envenenar el audio. Un `Fullscreen` inesperado puede
sorprender al usuario; si el archivo es de otra máquina, quizá quieras ignorarlo
la primera vez.

### Keybinds

Guarda el código numérico de `ebiten.Key` y valida que esté en un rango conocido.
Si cambias de versión y una tecla desaparece, usa el default para esa acción.

```go
func keyFromSave(v int, fallback ebiten.Key) ebiten.Key {
	if v < 0 || v > int(ebiten.KeyMax) {
		return fallback
	}
	return ebiten.Key(v)
}
```

Comprueba el nombre exacto de la constante en tu versión (`KeyMax` puede no
existir); si no, mantén una lista blanca de teclas permitidas.

## 8. Progreso y puntuaciones

- Desbloquea niveles por hitos claros («termina el nivel 1»), no por puntuación.
- Guarda la mejor puntuación por nivel y la fecha solo si aporta algo.
- Si hay continues, guárdalos como recurso, no como castigo permanente.
- No guardes la posición exacta del jugador en mitad de una oleada; al cargar,
  empieza en un punto seguro.
- Si el juego tiene múltiples perfiles, usa un archivo por perfil o una lista con
  un `profileID`; no mezcles progreso de dos jugadores.

## 9. Tests de guardado

```go
func TestSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "save.json")

	in := DefaultSave()
	in.Settings.MasterVolume = 0.5
	if err := writeAtomic(path, mustJSON(t, in)); err != nil {
		t.Fatal(err)
	}

	out, err := loadFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	if out.Settings.MasterVolume != 0.5 {
		t.Fatalf("volumen = %v", out.Settings.MasterVolume)
	}
}
```

Prueba también:

- Archivo inexistente → defaults.
- JSON corrupto → defaults + `.bak`.
- Versión antigua → migración.
- Valores fuera de rango → clamp.
- Escritura atómica no deja temporales.

No necesitas Ebitengine para estos tests; son funciones puras de disco/JSON.

## 10. Errores frecuentes

| Síntoma | Causa probable |
|---|---|
| El save se corrompe al cerrar | Escritura directa sin temporal |
| El juego no arranca tras editar el JSON | `Unmarshal` sin validación |
| Se pierde el progreso | Se guarda solo al cerrar y el cierre no es limpio |
| El volumen no se aplica | Se cargó pero no se aplicó a los players |
| El save no funciona en otra máquina | Rutas absolutas o assets referenciados por puntero |
| Se guarda cada frame | Falta debounce / dirty flag |
| El archivo crece sin control | Se guardan logs o estados temporales |

## 11. Ejercicio del capítulo

1. Define `SaveData`, `Settings` y `Progress` con `Version`.
2. Implementa `DefaultSave`, `Load`, `Save` y `writeAtomic`.
3. Guarda volumen, teclas, dificultad y niveles desbloqueados.
4. Aplica los ajustes al arrancar y comprueba que el audio cambia.
5. Simula un JSON corrupto y verifica que se crea `.bak` y el juego arranca.
6. Añade una migración de versión 1 a 2.
7. Escribe tests con `t.TempDir()` para round-trip, corrupto y migración.
8. Guarda al terminar una oleada y al cambiar un ajuste, no cada tick.

Criterio de aceptación: puedes borrar el save, jugar, cambiar el volumen, cerrar,
volver a abrir y encontrar el volumen y el progreso correctos.

## 12. Referencias

- `os.UserConfigDir`, `os.CreateTemp`, `os.Rename`, `os.ReadFile`.
- `encoding/json`.
- `examples/2048` para estado de juego y persistencia simple.
- `ebiten.SetFullscreen`, `ebiten.IsFullscreen`.
