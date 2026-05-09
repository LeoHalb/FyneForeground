package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("usage: filemover <destDir> <file1> [file2] ...")
	}

	dest := os.Args[1]
	for _, arg := range os.Args[2:] {
		// handle "dir/*" — move all children of dir
		if strings.HasSuffix(arg, "/*") {
			dir := strings.TrimSuffix(arg, "/*")
			entries, err := os.ReadDir(dir)
			if err != nil {
				log.Fatalf("error reading dir %s: %v\n", dir, err)
			}
			for _, entry := range entries {
				file := filepath.Join(dir, entry.Name())
				if err := move(file, dest); err != nil {
					log.Fatalf("error moving %s: %v\n", file, err)
				}
				fmt.Printf("moved %s -> %s\n", file, dest)
			}
			continue
		}

		if err := move(arg, dest); err != nil {
			log.Fatalf("error moving %s: %v\n", arg, err)
		}
		fmt.Printf("moved %s -> %s\n", arg, dest)
	}
}

func move(src, destDir string) error {
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", destDir, err)
	}

	dest := filepath.Join(destDir, filepath.Base(src))

	if info.IsDir() {
		destPath := filepath.Join(destDir, filepath.Base(src))
		_ = os.RemoveAll(destPath) // remove existing before copy
		if err := os.CopyFS(destPath, os.DirFS(src)); err != nil {
			return fmt.Errorf("copy dir %s -> %s: %w", src, destPath, err)
		}
		return os.RemoveAll(src)
	}

	in, err := os.Open(src)
	if err != nil {
		if in != nil {
			_ = in.Close()
		}
		return fmt.Errorf("open %s: %w", src, err)
	}

	out, err := os.Create(dest)
	if err != nil {
		_ = in.Close()
		return fmt.Errorf("create %s: %w", dest, err)
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, in); err != nil {
		_ = in.Close()
		return fmt.Errorf("copy %s -> %s: %w", src, dest, err)
	}

	err = in.Close()
	if err != nil {
		return fmt.Errorf("closing %s: %w", src, err)
	}

	return os.Remove(src)
}
