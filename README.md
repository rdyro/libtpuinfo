# libtpuinfo

A library for getting information about the TPU on a Google Cloud VM.

*Not an official Google product.*

```c
int (*tpu_chip_count)(void);
int (*tpu_metrics)(int port, int64 *device_ids, int64 *memory_usage, int64 *total_memory, double *duty_cycle_pct, int n);
int (*tpu_pids)(int64 *pids, int n);
```