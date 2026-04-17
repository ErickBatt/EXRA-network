package main

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/jaypipes/ghw"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetLastInputInfo = user32.NewProc("GetLastInputInfo")
	procGetTickCount     = kernel32.NewProc("GetTickCount")
)

type lastInputInfo struct {
	cbSize uint32
	dwTime uint32
}

type HardwareInfo struct {
	CPUModel    string   `json:"cpu_model"`
	CPUCores    int      `json:"cpu_cores"`
	RAMTotalGB  int      `json:"ram_total_gb"`
	GPUModels   []string `json:"gpu_models"`
	VRAMMB      int      `json:"vram_mb"`
	OS          string   `json:"os"`
	ComputeTier string   `json:"compute_tier"`
	
	// Dynamic metrics for "Power" engine
	CPULoad     float64  `json:"cpu_load"`
	RAMUsage    float64  `json:"ram_usage"`
	GPUUsage    float64  `json:"gpu_usage"` // Heuristic for now
	IsIdle      bool     `json:"is_idle"`
}

func GetHardwareInfo() *HardwareInfo {
	info := &HardwareInfo{
		OS:       runtime.GOOS,
		CPUCores: runtime.NumCPU(),
	}

	// 1. Static Info (CPU, RAM, GPU)
	cpus, err := cpu.Info()
	if err == nil && len(cpus) > 0 {
		info.CPUModel = cpus[0].ModelName
	}

	v, err := mem.VirtualMemory()
	if err == nil {
		info.RAMTotalGB = int(v.Total / 1024 / 1024 / 1024)
		info.RAMUsage = v.UsedPercent
	}

	gpu, err := ghw.GPU()
	if err == nil {
		for _, card := range gpu.GraphicsCards {
			if card.DeviceInfo != nil && card.DeviceInfo.Product != nil {
				info.GPUModels = append(info.GPUModels, card.DeviceInfo.Product.Name)
			}
		}
	}

	if runtime.GOOS == "windows" {
		info.VRAMMB = 8192 // Heuristic: will integrate WMI/DXGI in phase 3
	}

	// 2. Dynamic Info (Load & Idle)
	loads, err := load.Avg()
	if err == nil {
		info.CPULoad = loads.Load1
	}

	// Windows Idle Detection (Mouse/Keyboard inactivity)
	if runtime.GOOS == "windows" {
		info.IsIdle = detectWindowsIdle(5 * 60) // 5 minutes threshold
	}

	info.ComputeTier = calculateTier(info)
	return info
}

func calculateTier(info *HardwareInfo) string {
	base := "Compute-Low"
	if (len(info.GPUModels) > 0 || info.VRAMMB > 0) && info.RAMTotalGB >= 16 {
		base = "Compute-High"
	} else if info.CPUCores >= 8 && info.RAMTotalGB >= 8 {
		base = "Compute-Medium"
	}

	// If system is heavily loaded or NOT idle, downgrade tier to protect UX
	if info.CPULoad > 70.0 || !info.IsIdle {
		if base == "Compute-High" {
			return "Compute-Medium"
		}
		return "Compute-Low"
	}

	return base
}

func detectWindowsIdle(thresholdSeconds uint32) bool {
	if runtime.GOOS != "windows" {
		return true
	}
	var lastInput lastInputInfo
	lastInput.cbSize = uint32(unsafe.Sizeof(lastInput))
	ret, _, _ := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&lastInput)))
	if ret == 0 {
		return false
	}

	now, _, _ := procGetTickCount.Call()
	return (uint32(now) - lastInput.dwTime) > (thresholdSeconds * 1000)
}

func (h *HardwareInfo) Summary() string {
	gpuStr := "None"
	if len(h.GPUModels) > 0 {
		gpuStr = h.GPUModels[0]
	}
	idleStr := "Active"
	if h.IsIdle {
		idleStr = "Idle"
	}
	return fmt.Sprintf("OS: %s | CPU: %s (%d cores, %.1f%% load) | RAM: %d GB (%.1f%% used) | GPU: %s | Tier: %s | Mode: %s",
		h.OS, h.CPUModel, h.CPUCores, h.CPULoad, h.RAMTotalGB, h.RAMUsage, gpuStr, h.ComputeTier, idleStr)
}
