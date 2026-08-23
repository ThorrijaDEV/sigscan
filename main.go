package main

import (
	"flag"
	"fmt"
	"os"
)

const banner = ` ✦ SIGSCAN ✦ firma forense de ficheros — ¿qué escondes dentro?`

func main() {
	carve := flag.String("carve", "", "directorio donde tallar los ficheros embebidos")
	asJSON := flag.Bool("json", false, "salida JSON (una línea por fichero)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, banner)
		fmt.Fprintf(os.Stderr, "\nuso: sigscan [--carve DIR] [--json] FICHERO...\n\n")
		fmt.Fprintln(os.Stderr, "detecta JPEG/PNG/GIF/PDF/ZIP/GZIP/ELF embebidos por magic bytes")
		fmt.Fprintln(os.Stderr, "exit: 0 firmas halladas · 1 sin firmas · 2 error")
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	if !*asJSON {
		fmt.Println(banner)
	}
	os.Exit(run(os.Stdout, flag.Args(), *carve, *asJSON))
}
