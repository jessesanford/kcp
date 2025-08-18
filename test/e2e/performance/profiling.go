/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package performance

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"testing"
	"time"
)

// ProfileConfig configures profiling for benchmarks
type ProfileConfig struct {
	CPUProfile    bool   `json:"cpu_profile"`
	MemProfile    bool   `json:"mem_profile"`
	BlockProfile  bool   `json:"block_profile"`
	MutexProfile  bool   `json:"mutex_profile"`
	TraceEnabled  bool   `json:"trace_enabled"`
	OutputDir     string `json:"output_dir"`
	BenchmarkName string `json:"benchmark_name"`
}

// Profiler manages profiling for performance benchmarks
type Profiler struct {
	config      ProfileConfig
	cpuFile     *os.File
	traceFile   *os.File
	startTime   time.Time
	active      bool
}

// NewProfiler creates a new profiler with the given configuration
func NewProfiler(config ProfileConfig) *Profiler {
	if config.OutputDir == "" {
		config.OutputDir = "benchmark_profiles"
	}
	
	return &Profiler{
		config: config,
	}
}

// StartProfiling starts all enabled profiling modes
func (p *Profiler) StartProfiling(b *testing.B) error {
	if p.active {
		return fmt.Errorf("profiling already active")
	}

	p.startTime = time.Now()
	timestamp := p.startTime.Format("20060102_150405")
	profileDir := filepath.Join(p.config.OutputDir, fmt.Sprintf("%s_%s", p.config.BenchmarkName, timestamp))
	
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		return fmt.Errorf("failed to create profile directory: %w", err)
	}

	// Start CPU profiling
	if p.config.CPUProfile {
		cpuFile := filepath.Join(profileDir, "cpu.prof")
		f, err := os.Create(cpuFile)
		if err != nil {
			return fmt.Errorf("failed to create CPU profile file: %w", err)
		}
		p.cpuFile = f
		
		if err := pprof.StartCPUProfile(f); err != nil {
			f.Close()
			return fmt.Errorf("failed to start CPU profile: %w", err)
		}
		b.Logf("CPU profiling started: %s", cpuFile)
	}

	// Start execution trace
	if p.config.TraceEnabled {
		traceFile := filepath.Join(profileDir, "trace.out")
		f, err := os.Create(traceFile)
		if err != nil {
			return fmt.Errorf("failed to create trace file: %w", err)
		}
		p.traceFile = f
		
		if err := trace.Start(f); err != nil {
			f.Close()
			return fmt.Errorf("failed to start trace: %w", err)
		}
		b.Logf("Execution trace started: %s", traceFile)
	}

	// Configure block and mutex profiling
	if p.config.BlockProfile {
		runtime.SetBlockProfileRate(1)
		b.Logf("Block profiling enabled")
	}

	if p.config.MutexProfile {
		runtime.SetMutexProfileFraction(1)
		b.Logf("Mutex profiling enabled")
	}

	p.active = true
	return nil
}

// StopProfiling stops all active profiling and saves profiles
func (p *Profiler) StopProfiling(b *testing.B) error {
	if !p.active {
		return fmt.Errorf("profiling not active")
	}

	timestamp := p.startTime.Format("20060102_150405")
	profileDir := filepath.Join(p.config.OutputDir, fmt.Sprintf("%s_%s", p.config.BenchmarkName, timestamp))

	// Stop CPU profiling
	if p.config.CPUProfile && p.cpuFile != nil {
		pprof.StopCPUProfile()
		p.cpuFile.Close()
		p.cpuFile = nil
		b.Logf("CPU profiling stopped")
	}

	// Stop execution trace
	if p.config.TraceEnabled && p.traceFile != nil {
		trace.Stop()
		p.traceFile.Close()
		p.traceFile = nil
		b.Logf("Execution trace stopped")
	}

	// Save memory profile
	if p.config.MemProfile {
		memFile := filepath.Join(profileDir, "mem.prof")
		f, err := os.Create(memFile)
		if err != nil {
			return fmt.Errorf("failed to create memory profile file: %w", err)
		}
		defer f.Close()

		runtime.GC() // Force GC to get accurate heap profile
		if err := pprof.WriteHeapProfile(f); err != nil {
			return fmt.Errorf("failed to write memory profile: %w", err)
		}
		b.Logf("Memory profile saved: %s", memFile)
	}

	// Save block profile
	if p.config.BlockProfile {
		blockFile := filepath.Join(profileDir, "block.prof")
		f, err := os.Create(blockFile)
		if err != nil {
			return fmt.Errorf("failed to create block profile file: %w", err)
		}
		defer f.Close()

		if err := pprof.Lookup("block").WriteTo(f, 0); err != nil {
			return fmt.Errorf("failed to write block profile: %w", err)
		}
		runtime.SetBlockProfileRate(0) // Disable block profiling
		b.Logf("Block profile saved: %s", blockFile)
	}

	// Save mutex profile
	if p.config.MutexProfile {
		mutexFile := filepath.Join(profileDir, "mutex.prof")
		f, err := os.Create(mutexFile)
		if err != nil {
			return fmt.Errorf("failed to create mutex profile file: %w", err)
		}
		defer f.Close()

		if err := pprof.Lookup("mutex").WriteTo(f, 0); err != nil {
			return fmt.Errorf("failed to write mutex profile: %w", err)
		}
		runtime.SetMutexProfileFraction(0) // Disable mutex profiling
		b.Logf("Mutex profile saved: %s", mutexFile)
	}

	// Save goroutine profile
	goroutineFile := filepath.Join(profileDir, "goroutine.prof")
	f, err := os.Create(goroutineFile)
	if err != nil {
		return fmt.Errorf("failed to create goroutine profile file: %w", err)
	}
	defer f.Close()

	if err := pprof.Lookup("goroutine").WriteTo(f, 0); err != nil {
		return fmt.Errorf("failed to write goroutine profile: %w", err)
	}
	b.Logf("Goroutine profile saved: %s", goroutineFile)

	// Generate profile analysis
	if err := p.generateProfileAnalysis(profileDir, b); err != nil {
		b.Logf("Warning: failed to generate profile analysis: %v", err)
	}

	p.active = false
	return nil
}

// generateProfileAnalysis creates a summary analysis of the profiles
func (p *Profiler) generateProfileAnalysis(profileDir string, b *testing.B) error {
	analysisFile := filepath.Join(profileDir, "analysis.txt")
	f, err := os.Create(analysisFile)
	if err != nil {
		return fmt.Errorf("failed to create analysis file: %w", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "Performance Profile Analysis\n")
	fmt.Fprintf(f, "============================\n\n")
	fmt.Fprintf(f, "Benchmark: %s\n", p.config.BenchmarkName)
	fmt.Fprintf(f, "Timestamp: %s\n", p.startTime.Format(time.RFC3339))
	fmt.Fprintf(f, "Duration: %v\n", time.Since(p.startTime))
	fmt.Fprintf(f, "Go Version: %s\n", runtime.Version())
	fmt.Fprintf(f, "GOOS/GOARCH: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(f, "NumCPU: %d\n", runtime.NumCPU())
	fmt.Fprintf(f, "GOMAXPROCS: %d\n", runtime.GOMAXPROCS(0))
	fmt.Fprintf(f, "\n")

	// Memory statistics
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	fmt.Fprintf(f, "Memory Statistics:\n")
	fmt.Fprintf(f, "  Heap Alloc: %s\n", formatBytes(memStats.HeapAlloc))
	fmt.Fprintf(f, "  Heap Sys: %s\n", formatBytes(memStats.HeapSys))
	fmt.Fprintf(f, "  Heap Objects: %d\n", memStats.HeapObjects)
	fmt.Fprintf(f, "  Stack In Use: %s\n", formatBytes(memStats.StackInuse))
	fmt.Fprintf(f, "  Stack Sys: %s\n", formatBytes(memStats.StackSys))
	fmt.Fprintf(f, "  GC Cycles: %d\n", memStats.NumGC)
	fmt.Fprintf(f, "  GC Pause Total: %v\n", time.Duration(memStats.PauseTotalNs))
	if memStats.NumGC > 0 {
		fmt.Fprintf(f, "  GC Pause Avg: %v\n", time.Duration(memStats.PauseTotalNs)/time.Duration(memStats.NumGC))
	}
	fmt.Fprintf(f, "\n")

	// Goroutine count
	fmt.Fprintf(f, "Goroutines: %d\n", runtime.NumGoroutine())
	fmt.Fprintf(f, "\n")

	// Profile files generated
	fmt.Fprintf(f, "Profile Files Generated:\n")
	if p.config.CPUProfile {
		fmt.Fprintf(f, "  - cpu.prof (CPU profiling)\n")
	}
	if p.config.MemProfile {
		fmt.Fprintf(f, "  - mem.prof (Memory profiling)\n")
	}
	if p.config.BlockProfile {
		fmt.Fprintf(f, "  - block.prof (Blocking operations)\n")
	}
	if p.config.MutexProfile {
		fmt.Fprintf(f, "  - mutex.prof (Mutex contention)\n")
	}
	if p.config.TraceEnabled {
		fmt.Fprintf(f, "  - trace.out (Execution trace)\n")
	}
	fmt.Fprintf(f, "  - goroutine.prof (Goroutine dump)\n")
	fmt.Fprintf(f, "\n")

	fmt.Fprintf(f, "Analysis Commands:\n")
	if p.config.CPUProfile {
		fmt.Fprintf(f, "  CPU Profile: go tool pprof cpu.prof\n")
	}
	if p.config.MemProfile {
		fmt.Fprintf(f, "  Memory Profile: go tool pprof mem.prof\n")
	}
	if p.config.TraceEnabled {
		fmt.Fprintf(f, "  Execution Trace: go tool trace trace.out\n")
	}
	fmt.Fprintf(f, "\n")

	b.Logf("Profile analysis saved: %s", analysisFile)
	return nil
}

// ProfiledBenchmark wraps a benchmark function with profiling
func (p *Profiler) ProfiledBenchmark(b *testing.B, benchmarkFunc func(b *testing.B)) {
	// Start profiling
	if err := p.StartProfiling(b); err != nil {
		b.Fatalf("failed to start profiling: %v", err)
	}

	// Ensure profiling is stopped even if benchmark panics
	defer func() {
		if err := p.StopProfiling(b); err != nil {
			b.Errorf("failed to stop profiling: %v", err)
		}
	}()

	// Run the actual benchmark
	benchmarkFunc(b)
}

// formatBytes formats byte sizes in human readable format
func formatBytes(bytes uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// GetDefaultProfileConfig returns a default profiling configuration
func GetDefaultProfileConfig(benchmarkName string) ProfileConfig {
	return ProfileConfig{
		CPUProfile:    true,
		MemProfile:    true,
		BlockProfile:  false, // Can be expensive, enable for debugging
		MutexProfile:  false, // Can be expensive, enable for debugging
		TraceEnabled:  false, // Can generate large files, enable for detailed analysis
		BenchmarkName: benchmarkName,
		OutputDir:     "benchmark_profiles",
	}
}

// GetDetailedProfileConfig returns a configuration with all profiling enabled
func GetDetailedProfileConfig(benchmarkName string) ProfileConfig {
	return ProfileConfig{
		CPUProfile:    true,
		MemProfile:    true,
		BlockProfile:  true,
		MutexProfile:  true,
		TraceEnabled:  true,
		BenchmarkName: benchmarkName,
		OutputDir:     "benchmark_profiles",
	}
}