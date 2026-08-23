# ✦ SIGSCAN

Escáner forense de firmas mágicas en Go: encuentra JPEG/PNG/GIF/PDF/ZIP/GZIP/ELF embebidos dentro de cualquier fichero y los puede **tallar** (extraer) a disco. Solo stdlib de Go, binario estático, multiplataforma.

```
$ sigscan imagen_falsa.pdf
▸ imagen_falsa.pdf — 1 firma(s)
    @1024         pdf   45823 B

$ sigscan sospechoso.bin --carve ./extraído
▸ sospechoso.bin — 2 firma(s)
    @0            elf   2.3MiB
    @1843200      jpeg  1.4MB
    ↳ tallado → sospechoso.bin_000.elf
    ↳ tallado → sospechoso.bin_001.jpg
```

## Por qué

Herramientas como `binwalk` son potentes pero tienen dependencias C complejas. `sigscan` muestra que la detección de firmas básica —magic bytes con interpretación de estructuras simples (como el recorrido de chunks PNG o el footer JPEG) — cabe en ~180 líneas de Go puro. Ideal para análisis rápido de firmware, imágenes de memoria o ficheros sospechosos sin instalar nada.

## Uso

```bash
# instalar
go install github.com/ThorrijaDEV/sigscan@latest

# escanear
sigscan fichero [fichero...]
sigscan --carve DIRECTORIO fichero   # tallar hits con tamaño conocido
sigscan --json fichero               # salida JSON para pipes

# ejemplo forense
sigscan memoria.dump --carve ./hallazgos
```

Exit codes: `0` firmas halladas · `1` limpio · `2` error de lectura.

## Cómo funciona

- Recorre el fichero buscando los magic bytes de cada tipo.
- Para cada candidato, interpreta la estructura cuando exista (PNG recorre chunks hasta IEND, JPEG busca EOI, etc.) para determinar el tamaño total.
- Los hits con tamaño desconocido (por ausencia de footer) se reportan con `size: -1` y no se tallan.
- Salida legible o JSON línea a línea (un objeto por fichero analizado).
- Los ficheros tallados se nombran `basename_NNN.ext` dentro del directorio indicado.

## Estructura de datos

```golang
type Hit struct {
	Offset int64  // desde el inicio del fichero
	Kind   string // jpeg, png, gif, pdf, zip, gzip, elf
	Size   int64  // bytes, -1 si el footer es desconocido
}
```

## Testing

```bash
go test ./...   # fixtures sintéticos: PNG+JPEG embebidos, carving, límites
```

## Licencia

[MIT](LICENSE)
EOF