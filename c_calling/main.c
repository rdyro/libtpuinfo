#include <stdio.h>
#include <stdlib.h>
#include <dlfcn.h>

#define int64 long long

int main() {
    void *handle = dlopen("./lib/tpu_info_lib.so", RTLD_LAZY);
    if (!handle) {
        fprintf(stderr, "Error loading library: %s\n", dlerror());
        exit(EXIT_FAILURE);
    }
    int (*tpu_chip_count)(void) = dlsym(handle, "tpu_chip_count");
    int (*tpu_metrics)(int port, int64 *device_ids, int64 *memory_usage, int64 *total_memory, double *duty_cycle_pct, int n) = dlsym(handle, "tpu_metrics");
    int (*tpu_pids)(int64 *pids, int n) = dlsym(handle, "tpu_pids");
    
    int n = tpu_chip_count();
    printf("Chip count %d\n", n);

    int64 *pids = malloc(n * sizeof(int64));
    if (tpu_pids(pids, n) != 0) {
        printf("Error retrieving pids\n");    
        exit(1);
    }
    for (int i = 0; i < n; i ++) {
        printf("PID %lld\n", pids[i]);
    }
    
    int64 device_ids[32];
    int64 memory_usage[32];
    int64 total_memory[32];
    double duty_cycle_pct[32];
    
    // port <= 0 means default 8431 port
    if (tpu_metrics(-1, device_ids, memory_usage, total_memory, duty_cycle_pct, n) != 0) {
        printf("Error retrieving usage\n");
        fflush(stdout);
        exit(1);
    }
    for (int i = 0; i < n; i ++) {
        printf("%lld %lld %lld %.2f\n", device_ids[i], memory_usage[i], total_memory[i], duty_cycle_pct[i]);
    }
    
    return 0;
}
