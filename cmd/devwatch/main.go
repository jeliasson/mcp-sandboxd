package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
)

func main() {
	var pkg string
	var out string
	var debounce time.Duration
	flag.StringVar(&pkg, "pkg", "./cmd/mcp-sandboxd", "Go package to build")
	flag.StringVar(&out, "out", "bin/mcp-sandboxd", "Output binary path")
	flag.DurationVar(&debounce, "debounce", 300*time.Millisecond, "Restart debounce")
	flag.Parse()

	if pkg == "" {
		log.Fatal("-pkg is required")
	}
	if out == "" {
		log.Fatal("-out is required")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("watcher: %v", err)
	}
	defer w.Close()

	roots := []string{"cmd", "internal"}
	for _, r := range roots {
		_ = addRecursive(w, r)
	}
	_ = w.Add("go.mod")
	_ = w.Add("go.sum")

	var mu sync.Mutex
	var child *exec.Cmd

	rebuildAndRestart := func() {
		mu.Lock()
		defer mu.Unlock()

		if child != nil && child.Process != nil {
			_ = child.Process.Signal(syscall.SIGTERM)
			_, _ = child.Process.Wait()
			child = nil
		}

		if err := build(pkg, out); err != nil {
			log.Printf("build failed: %v", err)
			return
		}

		cmd := exec.CommandContext(ctx, out)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		cmd.Env = os.Environ()

		if err := cmd.Start(); err != nil {
			log.Printf("start failed: %v", err)
			return
		}
		child = cmd
		log.Printf("started %s (pid=%d)", out, cmd.Process.Pid)
	}

	rebuildAndRestart()

	var pending bool
	var timer *time.Timer
	schedule := func() {
		if debounce <= 0 {
			rebuildAndRestart()
			return
		}
		if timer == nil {
			timer = time.NewTimer(debounce)
			pending = true
			return
		}
		pending = true
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(debounce)
	}

	for {
		select {
		case <-stop:
			cancel()
			mu.Lock()
			if child != nil && child.Process != nil {
				_ = child.Process.Signal(syscall.SIGTERM)
				_, _ = child.Process.Wait()
			}
			mu.Unlock()
			return
		case ev := <-w.Events:
			if ev.Name == "" {
				continue
			}
			if !shouldTrigger(ev.Name) {
				continue
			}
			if ev.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) != 0 {
				// If a new directory is created, add watcher.
				info, err := os.Stat(ev.Name)
				if err == nil && info.IsDir() {
					_ = addRecursive(w, ev.Name)
				}
				schedule()
			}
		case <-func() <-chan time.Time {
			if timer == nil {
				return make(chan time.Time)
			}
			return timer.C
		}():
			if pending {
				pending = false
				rebuildAndRestart()
			}
		case err := <-w.Errors:
			log.Printf("watch error: %v", err)
		}
	}
}

func build(pkg, out string) error {
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	log.Printf("building %s -> %s", pkg, out)
	return cmd.Run()
}

func addRecursive(w *fsnotify.Watcher, root string) error {
	root = filepath.Clean(root)
	info, err := os.Stat(root)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		return nil
	}
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "bin" || base == "data" {
				return filepath.SkipDir
			}
			_ = w.Add(path)
		}
		return nil
	})
}

func shouldTrigger(path string) bool {
	base := filepath.Base(path)
	if strings.HasPrefix(base, ".") {
		return false
	}
	if strings.HasSuffix(path, "~") {
		return false
	}
	if strings.HasSuffix(path, ".go") || base == "go.mod" || base == "go.sum" {
		return true
	}
	return false
}
