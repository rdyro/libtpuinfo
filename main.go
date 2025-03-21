package main

import (
	"C"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"
	"unsafe"

	pb "github.com/rdyro/libtpuinfo/tpu_info_proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	TOTAL_MEMORY   = "tpu.runtime.hbm.memory.total.bytes"
	MEMORY_USAGE   = "tpu.runtime.hbm.memory.usage.bytes"
	DUTY_CYCLE_PCT = "tpu.runtime.tensorcore.dutycycle.percent"
)

const (
	googlePCIVendorID = "0x1ae0"
)

var (
	DebugEnabled bool        = os.Getenv("LIBTPUINFO_DEBUG") == "1"
	logger       *log.Logger = nil
)

func debugLogf(format string, args ...interface{}) {
	if logger == nil {
		logger = log.New(os.Stderr, "[libtpuinfo]: ", log.LstdFlags)
	}
	if DebugEnabled {
		logger.Printf(format, args...)
	}
}

var defaultGRPCPort int = parse_defaultGRPCPort()

func parse_defaultGRPCPort() int {
	// if not set, attempt to parse the environment variable
	defaultGRPCPort := 8431
	if env_port, is_set := os.LookupEnv("LIBTPUINFO_GRPC_PORT"); is_set {
		debugLogf("Found environment variable LIBTPUINFO_GRPC_PORT: %s\n", env_port)
		if val, err := strconv.ParseInt(env_port, 10, 64); err == nil {
			debugLogf("Parsed environment variable LIBTPUINFO_GRPC_PORT as int\n")
			defaultGRPCPort = int(val)
		}
	}
	return defaultGRPCPort
}

type TpuChipInfo struct {
	Name           string
	HBMGiB         int
	DevicesPerChip int
}

type TpuChip struct {
	Value TpuChipInfo
}

var (
	V2  = TpuChip{Value: TpuChipInfo{Name: "v2", HBMGiB: 8, DevicesPerChip: 2}}
	V3  = TpuChip{Value: TpuChipInfo{Name: "v3", HBMGiB: 16, DevicesPerChip: 2}}
	V4  = TpuChip{Value: TpuChipInfo{Name: "v4", HBMGiB: 32, DevicesPerChip: 1}}
	V5E = TpuChip{Value: TpuChipInfo{Name: "v5e", HBMGiB: 16, DevicesPerChip: 1}}
	V5P = TpuChip{Value: TpuChipInfo{Name: "v5p", HBMGiB: 95, DevicesPerChip: 1}}
	V6E = TpuChip{Value: TpuChipInfo{Name: "v6e", HBMGiB: 32, DevicesPerChip: 1}}
)

func (t TpuChip) String() string {
	return t.Value.Name
}

func fromPCIDeviceID(deviceID, subsystemID string) *TpuChip {
	// TPU v2 and v3 share a device ID
	if deviceID == "0x0027" {
		if subsystemID == "0x004e" {
			return &V2
		} else if subsystemID == "0x004f" {
			return &V3
		}
	}

	deviceIDToDevice := map[string]*TpuChip{
		"0x005e": &V4,
		"0x0063": &V5E,
		"0x0062": &V5P,
		"0x006f": &V6E,
	}

	if chip, ok := deviceIDToDevice[deviceID]; ok {
		return chip
	}
	return nil
}

// caching chip discovery
var (
	tpu_chip       (*TpuChip) = nil
	chip_count     int        = -1
	last_refreshed            = time.Now()
)

const cache_duration = 3 * time.Second

func isCacheValid() bool {
	return chip_count >= 0 && last_refreshed.After(time.Now().Add(-cache_duration))
}

func updateCache(chip *TpuChip, count int) {
	tpu_chip = chip
	chip_count = count
	last_refreshed = time.Now()
}

func getLocalChips() (*TpuChip, int) {
	if isCacheValid() {
		return tpu_chip, chip_count
	}
	cacheAndReturn := func(t *TpuChip, num int) (*TpuChip, int) {
		updateCache(t, num)
		return t, num
	}

	count := make(map[string]int)
	files, err := filepath.Glob("/sys/bus/pci/devices/*")
	if err != nil {
		return cacheAndReturn(nil, 0)
	}

	for _, pciPath := range files {
		vendorPath := filepath.Join(pciPath, "vendor")
		vendorIDBytes, err := ioutil.ReadFile(vendorPath)
		if err != nil {
			continue // Skip this device if we can't read the vendor ID
		}
		vendorID := string(vendorIDBytes[:len(vendorIDBytes)-1]) //remove newline
		if vendorID != googlePCIVendorID {
			continue
		}
		deviceIDPath := filepath.Join(pciPath, "device")
		deviceIDBytes, err := ioutil.ReadFile(deviceIDPath)
		if err != nil {
			continue // Skip this device
		}
		deviceID := string(deviceIDBytes[:len(deviceIDBytes)-1]) //remove newline

		subsystemPath := filepath.Join(pciPath, "subsystem_device")
		subsystemIDBytes, err := ioutil.ReadFile(subsystemPath)
		if err != nil {
			continue // Skip
		}
		subsystemID := string(subsystemIDBytes[:len(subsystemIDBytes)-1]) //remove newline

		chipType := fromPCIDeviceID(deviceID, subsystemID)
		if chipType != nil {
			count[chipType.Value.Name]++ //count by name instead of by *TpuChip
		}
	}

	if len(count) > 1 {
		panic(fmt.Sprintf("Expected one chip type, got %v", count))
	}
	if len(count) == 0 {
		return cacheAndReturn(nil, 0)
	}

	//find the only entry in count
	for name, num := range count {
		switch name {
		case "v2":
			return cacheAndReturn(&V2, num)
		case "v3":
			return cacheAndReturn(&V3, num)
		case "v4":
			return cacheAndReturn(&V4, num)
		case "v5e":
			return cacheAndReturn(&V5E, num)
		case "v5p":
			return cacheAndReturn(&V5P, num)
		case "v6e":
			return cacheAndReturn(&V6E, num)
		}
	}
	return cacheAndReturn(nil, 0)
}

func chipPath(chipType TpuChip, index int) string {
	if chipType == V5E || chipType == V5P || chipType == V6E {
		return fmt.Sprintf("/dev/vfio/%d", index)
	} else {
		return fmt.Sprintf("/dev/accel%d", index)
	}
}

var (
	tpuDeviceRegex = regexp.MustCompile(`^/dev/(?:accel|vfio/)\d+$`)
	procFDPidRegex = regexp.MustCompile(`/proc/(\d+)/fd/\d+`)
)

func getChipProcessOwners() (map[string]int64, error) {
	deviceOwners := make(map[string]int64)

	procDir := "/proc"
	pids, err := os.ReadDir(procDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc directory: %w", err)
	}
	for _, pidEntry := range pids {
		if !pidEntry.IsDir() {
			continue
		}

		pidStr := pidEntry.Name()
		pid, err := strconv.ParseInt(pidStr, 10, 64)
		if err != nil {
			continue // Not a PID directory, skip
		}
		fdDir := filepath.Join(procDir, pidStr, "fd")
		fdEntries, err := os.ReadDir(fdDir)
		if err != nil {
			// Process might have terminated or we don't have permissions to read /proc/<pid>/fd.
			if os.IsNotExist(err) || os.IsPermission(err) {
				continue // Skip to the next PID
			}
			return nil, fmt.Errorf("failed to read %s: %w", fdDir, err)
		}

		for _, fdEntry := range fdEntries {
			fdNumStr := fdEntry.Name()
			_, err := strconv.ParseInt(fdNumStr, 10, 64)
			if err != nil {
				continue // Not a file descriptor number, skip. Shouldn't really happen, but handle it.
			}
			fdLink := filepath.Join(fdDir, fdNumStr)
			file, err := os.Readlink(fdLink)
			if err != nil {
				// FileNotFoundError is expected if a process closes a file descriptor
				// while we're iterating.  Just ignore it.  Other errors are unexpected.
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("readlink failed for %s: %w", fdLink, err)
			}
			matched := tpuDeviceRegex.MatchString(file)
			if !matched {
				continue
			}
			deviceOwners[file] = pid
		}
	}
	return deviceOwners, nil
}

func getSortedMetrics[T any](r *pb.MetricResponse, get_value func(g *pb.Gauge) T) ([]int, []T) {
	metrics := r.GetMetric().GetMetrics()
	metric_map := make(map[int]T)
	device_ids := make([]int, 0)
	for _, m := range metrics {
		key := m.GetAttribute().GetValue().GetIntAttr()
		val := get_value(m.GetGauge())
		device_ids = append(device_ids, int(key))
		metric_map[int(key)] = val
	}
	sort.Ints(device_ids)
	metric_list := make([]T, len(metrics))
	for i, k := range device_ids {
		metric_list[i] = metric_map[k]
	}
	return device_ids, metric_list
}

func copyValuesToC[T1 any, T2 any](c_array_ *T1, go_array []T2, convertFn func(T2) T1) {
	c_array := (*[1 << 30]T1)(unsafe.Pointer(c_array_))[:len(go_array):len(go_array)]
	for i, v := range go_array {
		c_array[i] = convertFn(v)
	}
}

//export tpu_chip_count
func tpu_chip_count() C.int {
	chip_type, count := getLocalChips()
	if chip_type != nil {
		count = count * (*chip_type).Value.DevicesPerChip
	}
	return C.int(count)
}

//export tpu_pids
func tpu_pids(pids *C.longlong, n C.int) C.int {
	chip_type, count := getLocalChips()
	devices_per_chip := 1
	if chip_type != nil {
		devices_per_chip = (*chip_type).Value.DevicesPerChip
		count = count * devices_per_chip
	}
	if count != int(n) {
		debugLogf("Requested PIDs for %d TPU chips, but only %d found\n", n, count)
		return 1
	}
	chip_owners, err := getChipProcessOwners()
	if err != nil || len(chip_owners)*devices_per_chip != int(n) {
		debugLogf("Could not find TPU processes.")
		if err != nil {
			debugLogf(" %v\n", err)
		} else {
			debugLogf(" Asked for %d chips, but found %d processes\n", int(n), len(chip_owners))
		}
		return 2
	}
	pids_go := (*[1 << 30]C.longlong)(unsafe.Pointer(pids))[:count:count]
	chip_paths := make([]string, 0)
	for path, _ := range chip_owners {
		chip_paths = append(chip_paths, path)
	}
	sort.Strings(chip_paths)
	for i, path := range chip_paths {
		for j := 0; j < devices_per_chip; j++ {
			pids_go[devices_per_chip*i+j] = C.longlong(chip_owners[path])
		}
	}
	return 0
}

//export tpu_metrics
func tpu_metrics(port C.int, device_ids_ *C.longlong, memory_usage_ *C.longlong, total_memory_ *C.longlong, duty_cycle_pct_ *C.double, n C.int) C.int {

	chip_type, count := getLocalChips()
	if chip_type != nil {
		count = count * (*chip_type).Value.DevicesPerChip
	}
	if count != int(n) {
		debugLogf("Requested metrics for %d TPU chips, but only %d found\n", n, count)
		return 1
	}
	if int(n) == 0 {
		return 0
	}
	if port <= 0 {
		port = C.int(defaultGRPCPort)
	}
	addr := fmt.Sprintf("localhost:%d", port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		debugLogf("Could not connect to the TPU metrics GRPC server: %v\n", err)
		return 1
	}
	defer conn.Close()
	c := pb.NewRuntimeMetricServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// get the metrics
	r, err := c.GetRuntimeMetric(ctx, &pb.MetricRequest{MetricName: MEMORY_USAGE})
	if err != nil {
		debugLogf("Could not get MEMORY_USAGE metrics: %v\n", err)
		return 2
	}
	device_ids, memory_usage := getSortedMetrics(r, func(x *pb.Gauge) int64 { return x.GetAsInt() })
	if count != len(device_ids) {
		debugLogf("%d metrics found, but that doesn't match the discovered number of chips: %d\n", len(device_ids), count)
		return 2
	}

	// check for number of metric agreement early before checking other metrics
	if int(n) != len(device_ids) {
		debugLogf("Asked for metrics for %d chips, but %d chips found\n", int(n), len(device_ids))
		return 2
	}

	r, err = c.GetRuntimeMetric(ctx, &pb.MetricRequest{MetricName: TOTAL_MEMORY})
	if err != nil {
		debugLogf("Could not get TOTAL_MEMORY metrics: %v\n", err)
		return 2
	}
	_, total_memory := getSortedMetrics(r, func(x *pb.Gauge) int64 { return x.GetAsInt() })

	r, err = c.GetRuntimeMetric(ctx, &pb.MetricRequest{MetricName: DUTY_CYCLE_PCT})
	if err != nil {
		debugLogf("Could not get DUTY_CYCLE_PCT metrics: %v\n", err)
		return 2
	}
	_, duty_cycle_pct := getSortedMetrics(r, func(x *pb.Gauge) float64 { return x.GetAsDouble() })

	// Duty cycle is always measured per-chip, while memory is measured per-core.
	// Repeat if necessary so these responses are the same length.
	cores_per_chip := len(total_memory) / len(duty_cycle_pct)
	duty_cycle_per_core_pct := make([]float64, len(total_memory))

	for i := 0; i < len(duty_cycle_pct); i++ {
		for j := 0; j < cores_per_chip; j++ {
			duty_cycle_per_core_pct[cores_per_chip*i+j] = duty_cycle_pct[i]
		}
	}
	// check that the info length matches for all statistics
	if len(device_ids) != len(memory_usage) || len(total_memory) != len(memory_usage) || len(memory_usage) != len(duty_cycle_per_core_pct) {
		debugLogf("Lengths of metrics do not agree. len(total_memory) = %d; len(memory_usage) = %d; len(duty_cycle_per_core_pct) = %d\n",
			len(total_memory), len(memory_usage), len(duty_cycle_per_core_pct))
		return 3
	}

	copyValuesToC(device_ids_, device_ids, func(a int) C.longlong { return C.longlong(a) })
	copyValuesToC(memory_usage_, memory_usage, func(a int64) C.longlong { return C.longlong(a) })
	copyValuesToC(total_memory_, total_memory, func(a int64) C.longlong { return C.longlong(a) })
	copyValuesToC(duty_cycle_pct_, duty_cycle_per_core_pct, func(a float64) C.double { return C.double(a) })

	return 0
}

type Metrics struct {
	DeviceIDs    []int
	MemoryUsage  []int64
	TotalMemory  []int64
	DutyCyclePct []float64
}

func GetMetrics() *Metrics {

	// connect to the server
	addr := "localhost:8431"
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		debugLogf("Could not connect to the GRPC server: %v\n", err)
		return nil
	}
	defer conn.Close()
	c := pb.NewRuntimeMetricServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// get the metrics
	r, err := c.GetRuntimeMetric(ctx, &pb.MetricRequest{MetricName: MEMORY_USAGE})
	if err != nil {
		debugLogf("Could not get MEMORY_USAGE metrics: %v\n", err)
		return nil
	}
	device_ids, memory_usage := getSortedMetrics(r, func(x *pb.Gauge) int64 { return x.GetAsInt() })

	r, err = c.GetRuntimeMetric(ctx, &pb.MetricRequest{MetricName: TOTAL_MEMORY})
	if err != nil {
		debugLogf("Could not get TOTAL_MEMORY metrics: %v\n", err)
		return nil
	}
	_, total_memory := getSortedMetrics(r, func(x *pb.Gauge) int64 { return x.GetAsInt() })

	r, err = c.GetRuntimeMetric(ctx, &pb.MetricRequest{MetricName: DUTY_CYCLE_PCT})
	if err != nil {
		debugLogf("Could not get DUTY_CYCLE_PCT metrics: %v\n", err)
		return nil
	}
	_, duty_cycle_pct := getSortedMetrics(r, func(x *pb.Gauge) float64 { return x.GetAsDouble() })

	// Duty cycle is always measured per-chip, while memory is measured per-core.
	// Repeat if necessary so these responses are the same length.
	cores_per_chip := len(total_memory) / len(duty_cycle_pct)
	duty_cycle_per_core_pct := make([]float64, len(total_memory))
	for i := 0; i < len(duty_cycle_pct); i++ {
		for j := 0; j < cores_per_chip; j++ {
			duty_cycle_per_core_pct[cores_per_chip*i+j] = duty_cycle_pct[i]
		}
	}

	// check that the info length matches for all statistics
	if len(device_ids) != len(memory_usage) || len(total_memory) != len(memory_usage) || len(memory_usage) != len(duty_cycle_per_core_pct) {
		debugLogf(`Lengths of metrics do not agree. len(total_memory) = %d; 
								len(memory_usage) = %d; len(duty_cycle_per_core_pct) = %d\n`,
			len(total_memory), len(memory_usage), len(duty_cycle_per_core_pct))
		return nil
	}

	return &Metrics{device_ids, memory_usage, total_memory, duty_cycle_per_core_pct}
}

func main() {
	DebugEnabled = true

	t := time.Now()
	chip_type, count := getLocalChips()
	debugLogf("Finding chips takes %d us\n", time.Since(t).Microseconds())

	debugLogf("Found %d chips\n", count)

	pid := int64(-1)
	for i := 0; i < 10; i++ {
		t = time.Now()
		chip_owners, err := getChipProcessOwners()
		debugLogf("Finding chip process owners takes %d us\n", time.Since(t).Microseconds())
		if err != nil {
			debugLogf("Could not get chip owners: %v\n", err)
			os.Exit(1)
		}
		for _, k := range chip_owners {
			pid = k
			break
		}
	}

	for i := 0; i < 10; i++ {
		t = time.Now()
		metrics_ptr := GetMetrics()
		if metrics_ptr == nil {
			debugLogf("Could not get metrics\n")
			debugLogf("Getting metrics takes %d us\n", time.Since(t).Microseconds())
		}
		if metrics_ptr != nil {
			metrics := *metrics_ptr
			// fmt.Printf("Device Id Memory Usage Total Memory Duty Cycle Pct\n")  // skip header
			for i, _ := range metrics.DeviceIDs {
				debugLogf("%d %d %d %.2f %s %d\n", metrics.DeviceIDs[i], metrics.MemoryUsage[i], metrics.TotalMemory[i],
					metrics.DutyCyclePct[i], chip_type.Value.Name, pid)
			}
		}
	}
}
