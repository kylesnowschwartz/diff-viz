package diff

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// BenchmarkCaptureCurrentTree measures CaptureCurrentTree with N untracked files.
//
// Baseline (2025-01, Apple M3 Max): ~80ms constant regardless of file count.
// Before optimization: 305ms/10 files, 1050ms/50 files, 1984ms/100 files (O(N) process spawns).
//
// Run with: go test -bench=BenchmarkCaptureCurrentTree -benchtime=3s ./diff/
func BenchmarkCaptureCurrentTree(b *testing.B) {
	for _, numFiles := range []int{10, 50, 100} {
		b.Run(fmt.Sprintf("untracked=%d", numFiles), func(b *testing.B) {
			// Create temp repo
			tmpDir, err := os.MkdirTemp("", "bench-capture-tree-*")
			if err != nil {
				b.Fatal(err)
			}
			defer os.RemoveAll(tmpDir)

			// Initialize git repo
			run := func(args ...string) {
				cmd := exec.Command("git", args...)
				cmd.Dir = tmpDir
				if out, err := cmd.CombinedOutput(); err != nil {
					b.Fatalf("git %v failed: %v\n%s", args, err, out)
				}
			}

			run("init")
			run("config", "user.email", "bench@test.com")
			run("config", "user.name", "Benchmark")

			// Create initial commit
			initialFile := filepath.Join(tmpDir, "README.md")
			if err := os.WriteFile(initialFile, []byte("# Test\n"), 0644); err != nil {
				b.Fatal(err)
			}
			run("add", "README.md")
			run("commit", "-m", "initial")

			// Create N untracked files
			for i := 0; i < numFiles; i++ {
				path := filepath.Join(tmpDir, fmt.Sprintf("untracked_%03d.txt", i))
				if err := os.WriteFile(path, []byte(fmt.Sprintf("file %d\n", i)), 0644); err != nil {
					b.Fatal(err)
				}
			}

			// Save original dir, switch to temp repo
			origDir, _ := os.Getwd()
			os.Chdir(tmpDir)
			defer os.Chdir(origDir)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := CaptureCurrentTree()
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
