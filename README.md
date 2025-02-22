# libtpuinfo

A library for getting information about the TPU on a Google Cloud VM.

*Not an official Google product.*

## API

Available C symbols are:
```c
// Get the number of TPU chips on the VM
int (*tpu_chip_count)(void);

// Get the process IDs for all `n` TPU devices
int (*tpu_pids)(int64 *pids, int n);

// Get the metrics for all `n` TPU devices
int (*tpu_metrics)(int port, int64 *device_ids, int64 *memory_usage, int64 *total_memory, double *duty_cycle_pct, int n);
```

## Installation

Download your architecture specific library from [releases](https://github.com/rdyro/libtpuinfo/releases) and install 
```bash
cp libtpuinfo-linux-x86_64.so /usr/local/lib/
```

### Building from source

```bash
git clone https://github.com/GoogleCloudPlatform/libtpuinfo.git && cd libtpuinfo
make
sudo make install
```

or manually

```bash
git clone https://github.com/GoogleCloudPlatform/libtpuinfo.git
go build -buildmode=c-shared -o ${LIB} main.go
sudo cp libtpuinfo.so /usr/local/lib/
```

## Testing usage from C

```bash
make test
```
