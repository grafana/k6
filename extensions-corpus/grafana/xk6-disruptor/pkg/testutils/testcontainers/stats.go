// Package testcontainers implements utility functions for running tests with TestContainers
package testcontainers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
)

// ContainerStats defines a summary of the container's starts
type ContainerStats struct {
	Timestamp         time.Time
	CPUUsageTotal     uint64
	CPUUsageInKernel  uint64
	CPUUsageUser      uint64
	CPUPercentage     float64
	MemoryUsage       uint64
	MemoryMaxUsage    uint64
	MemoryLimit       uint64
	MemoryPercentage  float64
	NetworkRxBytes    uint64
	NetworkTxBytes    uint64
	BlockIOReadBytes  uint64
	BlockIOWriteBytes uint64
}

// Calculate the memory usage discounting the cache size.
// Depending on whether docker is running with cgroups V1 or V2, cache size is reported differently
func calculateMemoryUsage(memory container.MemoryStats) uint64 {
	// check groups v1 format
	if cache, isCgroup1 := memory.Stats["total_inactive_file"]; isCgroup1 && cache < memory.Usage {
		return memory.Usage - cache
	}

	if cache := memory.Stats["inactive_file"]; cache < memory.Usage {
		return memory.Usage - cache
	}

	return memory.Usage
}

// collectLinuxStats Collects stats for a Linux system using a current sample and a base sample.
// Base sample is used for calculating the CPU percentage. If the base sample is empty, CPU percentage is reported as 0%
func collectLinuxStats(base, sample container.StatsResponse) ContainerStats {
	stats := ContainerStats{}
	stats.Timestamp = sample.Read

	// CPU stats
	stats.CPUUsageTotal = sample.CPUStats.CPUUsage.TotalUsage
	stats.CPUUsageInKernel = sample.CPUStats.CPUUsage.UsageInKernelmode
	stats.CPUUsageUser = sample.CPUStats.CPUUsage.UsageInUsermode

	// usage stats are counters, so percentage is calculated over the delta of two samples
	deltaUsage := float64(sample.CPUStats.CPUUsage.TotalUsage - base.CPUStats.CPUUsage.TotalUsage)
	deltaSystemUsage := float64(sample.CPUStats.SystemUsage - base.CPUStats.SystemUsage)
	if deltaSystemUsage > 0 && deltaUsage > 0 {
		stats.CPUPercentage = deltaUsage / deltaSystemUsage * float64(sample.CPUStats.OnlineCPUs) * 100.0
	}

	// memory stats
	stats.MemoryUsage = calculateMemoryUsage(sample.MemoryStats)
	stats.MemoryMaxUsage = sample.MemoryStats.MaxUsage
	stats.MemoryLimit = sample.MemoryStats.Limit
	if sample.MemoryStats.Limit != 0 {
		stats.MemoryPercentage = float64(stats.MemoryUsage) / float64(sample.MemoryStats.Limit) * 100.0
	}

	// aggregate block I/O states
	for _, dev := range sample.BlkioStats.IoServiceBytesRecursive {
		switch dev.Op {
		case "read":
			stats.BlockIOReadBytes += dev.Value
		case "write":
			stats.BlockIOWriteBytes += dev.Value
		}
	}

	// aggregate network stats
	for _, n := range sample.Networks {
		stats.NetworkRxBytes += n.RxBytes
		stats.NetworkTxBytes += n.TxBytes
	}

	return stats
}

// sampleStats gets a one shot sample of stats. Returns the sample data and the OS type
func sampleStats(ctx context.Context, containerID string) (container.StatsResponse, error) {
	provider, err := testcontainers.NewDockerProvider()
	if err != nil {
		return container.StatsResponse{}, fmt.Errorf("getting docker provider %w", err)
	}

	resp, err := provider.Client().ContainerStatsOneShot(ctx, containerID)
	if err != nil {
		return container.StatsResponse{}, fmt.Errorf("requesting stats %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.OSType != "linux" {
		return container.StatsResponse{}, fmt.Errorf("unsupported OS: %s", resp.OSType)
	}

	buffer := bytes.Buffer{}
	_, err = buffer.ReadFrom(resp.Body)
	if err != nil {
		return container.StatsResponse{}, fmt.Errorf("reading stats %w", err)
	}

	statsData := container.StatsResponse{}
	err = json.Unmarshal(buffer.Bytes(), &statsData)
	if err != nil {
		return container.StatsResponse{}, fmt.Errorf("unmarshalling stats %w", err)
	}

	return statsData, nil
}

// Stats works as Docker Stats command and retrieves a summary of container resource usage.
// As CPU measurement are accumulated, in order to calculate CPU percentage, two samples are
// taken a second apart and the incremental usage is used for estimating the percentage.
func Stats(ctx context.Context, containerID string) (ContainerStats, error) {
	base, err := sampleStats(ctx, containerID)
	if err != nil {
		return ContainerStats{}, err
	}

	// FIXME: we need two samples in order to calculate the CPU percentage.
	// It doesn't feel right hardcoding this time
	time.Sleep(time.Second)

	sample, err := sampleStats(ctx, containerID)
	if err != nil {
		return ContainerStats{}, err
	}

	return collectLinuxStats(base, sample), nil
}
