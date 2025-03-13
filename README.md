# libtpuinfo

A library for getting information about the TPU on a Google Cloud VM built by
transcribing
[tpu-info](https://github.com/AI-Hypercomputer/cloud-accelerator-diagnostics/tree/main/tpu_info)
into golang.

## API

Available C symbols are:
```c
// Get the number of TPU chips on the VM
int (*tpu_chip_count)(void);

// Get the process IDs for all `n` TPU devices
int (*tpu_pids)(long long *pids, int n);

// Get the metrics for all `n` TPU devices (port <= 0 implies default 8431)
int (*tpu_metrics)(int port, long long *device_ids, long long *memory_usage, 
                   long long *total_memory, double *duty_cycle_pct, int n);
```

## Installation

Download your architecture specific library from [releases](https://github.com/rdyro/libtpuinfo/releases) and install 
```bash
cp libtpuinfo-linux-x86_64.so /usr/local/lib/libtpuinfo.so
# or
cp libtpuinfo-linux-aarch64.so /usr/local/lib/libtpuinfo.so
```

### Building from source

```bash
git clone https://github.com/rdyro/libtpuinfo.git && cd libtpuinfo
make
sudo make install
```

or manually

```bash
git clone https://github.com/rdyro/libtpuinfo.git
go build -buildmode=c-shared -o libtpuinfo.so main.go
sudo cp libtpuinfo.so /usr/local/lib/
```

## Testing usage from C

```bash
make test
```


## Environment variables

- `LIBTPUINFO_DEBUG=1` will enable debug logging.
- `LIBTPUINFO_GRPC_PORT={int}` will set the port for the default gRPC server for TPU runtime metrics.
