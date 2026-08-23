package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Hit es una firma encontrada dentro de un fichero.
type Hit struct {
	Offset int64  `json:"offset"`
	Kind   string `json:"kind"`
	Size   int64  `json:"size"` // -1 = sin footer conocido
}

type sig struct {
	kind   string
	magic  []byte
	footer []byte // opcional: fin del contenedor
	footIn bool   // el footer se incluye en el tamaño (IEND+CRC)
	minLen int
}

var signatures = []sig{
	{"jpeg", []byte{0xFF, 0xD8, 0xFF}, []byte{0xFF, 0xD9}, true, 128},
	{"png", []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A},
		nil, false, 8}, // footer especial IEND, ver scanFile
	{"gif", []byte("GIF89a"), nil, false, 14},
	{"gif", []byte("GIF87a"), nil, false, 14},
	{"pdf", []byte("%PDF-"), []byte("%%EOF"), false, 100},
	{"zip", []byte{'P', 'K', 0x03, 0x04}, nil, false, 22},
	{"gzip", []byte{0x1F, 0x8B}, nil, false, 18},
	{"elf", []byte{0x7F, 'E', 'L', 'F'}, nil, false, 64},
}

const pngFooter = "IEND"

// ScanFile lee todo el path y devuelve las firmas detectadas,
// ordenadas por offset.
func ScanFile(path string) ([]Hit, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return scanBytes(data), nil
}

func scanBytes(data []byte) []Hit {
	var hits []Hit
	for _, s := range signatures {
		for off := 0; off+len(s.magic) <= len(data); {
			idx := bytes.Index(data[off:], s.magic)
			if idx < 0 {
				break
			}
			at := off + idx
			size := int64(-1)
			if end := findEnd(data, at, s); end > 0 {
				size = end
			}
			hits = append(hits, Hit{Offset: int64(at), Kind: s.kind, Size: size})
			off = at + len(s.magic)
		}
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Offset < hits[j].Offset })
	return hits
}

// findEnd localiza el footer desde at y devuelve el tamaño total.
// PNG usa la estructura de chunks (IEND + CRC de 4 bytes).
func findEnd(data []byte, at int, s sig) int64 {
	switch {
	case s.footer != nil:
		idx := bytes.Index(data[at:], s.footer)
		if idx < 0 {
			return -1
		}
		end := at + idx + len(s.footer)
		if s.footIn && s.kind == "jpeg" {
			return int64(end - at)
		}
		return int64(end - at)
	case s.kind == "png":
		// recorrer chunks desde después de la firma (8 bytes)
		pos := at + 8
		for pos+8 <= len(data) {
			length := binary.BigEndian.Uint32(data[pos : pos+4])
			typ := string(data[pos+4 : pos+8])
			pos += 12 + int(length)
			if typ == pngFooter && pos <= len(data) {
				return int64(pos - at)
			}
		}
		return -1
	default:
		return -1
	}
}

// Carve extrae los hits con tamaño conocido a outDir.
func Carve(data []byte, hits []Hit, outDir, base string) ([]string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}
	var written []string
	ext := map[string]string{"jpeg": ".jpg", "png": ".png", "pdf": ".pdf"}
	n := 0
	for _, h := range hits {
		if h.Size < 0 {
			continue
		}
		e := ext[h.Kind]
		if e == "" {
			continue // solo tipos contenedores conocidos
		}
		name := filepath.Join(outDir, fmt.Sprintf("%s_%03d%s", base, n, e))
		end := h.Offset + h.Size
		if end > int64(len(data)) {
			end = int64(len(data))
		}
		if err := os.WriteFile(name, data[h.Offset:end], 0o644); err != nil {
			return written, err
		}
		written = append(written, name)
		n++
	}
	return written, nil
}

// run procesa cada path y escribe el informe en w. Devuelve código de salida.
func run(w io.Writer, paths []string, carveDir string, asJSON bool) int {
	total := 0
	encoder := json.NewEncoder(w)
	for _, p := range paths {
		hits, err := ScanFile(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sigscan: %v\n", err)
			return 2
		}
		total += len(hits)
		if asJSON {
			_ = encoder.Encode(map[string]any{"file": p, "hits": hits})
			continue
		}
		fmt.Printf("▸ %s — %d firma(s)\n", p, len(hits))
		for _, h := range hits {
			sizeTxt := "¿?"
			if h.Size >= 0 {
				sizeTxt = fmt.Sprintf("%d B", h.Size)
			}
			fmt.Printf("    @%-10d %-6s %s\n", h.Offset, h.Kind, sizeTxt)
		}
		if len(hits) == 0 {
			fmt.Println("    limpia")
		}
		if carveDir != "" {
			data, _ := os.ReadFile(p)
			files, err := Carve(data, hits, carveDir, filepath.Base(p))
			if err != nil {
				fmt.Fprintf(os.Stderr, "sigscan: carving: %v\n", err)
				return 2
			}
			for _, f := range files {
				fmt.Printf("    ↳ tallado → %s\n", f)
			}
		}
	}
	if total == 0 {
		return 1
	}
	return 0
}
