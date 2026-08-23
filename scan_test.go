package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// pngBlob construye un PNG mínimo válido: firma + IHDR + IEND.
func pngBlob() []byte {
	b := new(bytes.Buffer)
	b.Write([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A})
	chunk := func(typ string, payload []byte) {
		var l [4]byte
		binary.BigEndian.PutUint32(l[:], uint32(len(payload)))
		b.Write(l[:])
		b.WriteString(typ)
		b.Write(payload)
		b.Write([]byte{0xDE, 0xAD, 0xBE, 0xEF}) // CRC falso pero de 4 bytes
	}
	chunk("IHDR", make([]byte, 13))
	chunk("IEND", nil)
	return b.Bytes()
}

// jpegBlob: SOI + relleno + EOI.
func jpegBlob() []byte {
	b := []byte{0xFF, 0xD8, 0xFF, 0xE0}
	return append(b, append(make([]byte, 200), 0xFF, 0xD9)...)
}

func embed(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
		out = append(out, 0xAA, 0xBB, 0xCC) // separador basura
	}
	return out
}

func writeTmp(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "blob.bin")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScanOffsets(t *testing.T) {
	png := pngBlob()
	jpg := jpegBlob()
	data := embed(png, jpg)
	path := writeTmp(t, data)

	hits, err := ScanFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("esperaba 2 hits, hubo %d: %+v", len(hits), hits)
	}
	if hits[0].Kind != "png" || hits[0].Offset != 0 {
		t.Errorf("hit0 = %+v", hits[0])
	}
	// tamaño PNG: firma(8)+chunk IHDR(4+4+13+4=25)+chunk IEND(12)=45
	wantPngSize := int64(len(png))
	if hits[0].Size != wantPngSize {
		t.Errorf("tamaño PNG = %d, want %d", hits[0].Size, wantPngSize)
	}
	if hits[1].Kind != "jpeg" {
		t.Errorf("hit1.kind = %s", hits[1].Kind)
	}
	if hits[1].Offset <= hits[0].Offset {
		t.Errorf("orden incorrecto: %+v", hits)
	}
}

func TestScanCleanFile(t *testing.T) {
	path := writeTmp(t, bytes.Repeat([]byte{0x42}, 500))
	hits, err := ScanFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Errorf("fichero limpio con hits: %+v", hits)
	}
}

func TestJpegFooterSize(t *testing.T) {
	jpg := jpegBlob() // 4 + 200 + 2 = 206
	data := append(bytes.Repeat([]byte{0x00}, 37), jpg...)
	path := writeTmp(t, data)
	hits, _ := ScanFile(path)
	if len(hits) != 1 || hits[0].Kind != "jpeg" {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].Offset != 37 || hits[0].Size != int64(len(jpg)) {
		t.Errorf("jpeg @%d size %d; want @37 size %d", hits[0].Offset, hits[0].Size, len(jpg))
	}
}

func TestCarveExtractsBytes(t *testing.T) {
	png := pngBlob()
	data := embed(bytes.Repeat([]byte{0x01}, 10), png)
	path := writeTmp(t, data)
	hits, _ := ScanFile(path)
	outDir := t.TempDir()
	files, err := Carve(data, hits, outDir, "origen")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("carve produjo %d ficheros", len(files))
	}
	got, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, png) {
		t.Errorf("carve no reproduce el PNG original (%d vs %d bytes)", len(got), len(png))
	}
}

func TestCarveSkipsUnknownSize(t *testing.T) {
	zipMagic := []byte{'P', 'K', 0x03, 0x04} // sin estructura: size -1
	hits := []Hit{{Offset: 5, Kind: "zip", Size: -1}}
	files, err := Carve(nil, hits, t.TempDir(), "x")
	if err != nil || len(files) != 0 {
		t.Errorf("debería saltar size desconocido: %v %v", files, err)
	}
	_ = zipMagic
}

func TestRunExitCodes(t *testing.T) {
	clean := writeTmp(t, bytes.Repeat([]byte{0x00}, 100))
	full := writeTmp(t, embed(jpegBlob()))
	if code := run(&bytes.Buffer{}, []string{clean}, "", false); code != 1 {
		t.Errorf("limpio → exit %d, want 1", code)
	}
	if code := run(&bytes.Buffer{}, []string{full}, "", false); code != 0 {
		t.Errorf("con firmas → exit %d, want 0", code)
	}
	if code := run(&bytes.Buffer{}, []string{"/no/existe/xx"}, "", false); code != 2 {
		t.Errorf("inexistente → exit %d, want 2", code)
	}
}
