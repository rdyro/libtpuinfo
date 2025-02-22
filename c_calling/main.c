#include <stdio.h>
#include <stdlib.h>
#include <dlfcn.h>

#define int64 long long

int (*tpu_chip_count)(void);
int (*tpu_metrics)(int port, int64 *device_ids, int64 *memory_usage, int64 *total_memory, double *duty_cycle_pct, int n);
int (*tpu_pids)(int64 *pids, int n);

char *libname = "libtpuinfo.so";

int resolve_symbols() {
    char* error_msg;
    char symbol_name[64];
    char library_path[256];

    // Load the library
    snprintf(library_path, sizeof(library_path), "%s", libname);
    void *handle = dlopen(library_path, RTLD_LAZY);
    if (!handle) {
        fprintf(stderr, "Error loading library: %s\n", dlerror());
        return 1;
    }

    snprintf(symbol_name, sizeof(symbol_name), "%s", "tpu_chip_count");
    tpu_chip_count = dlsym(handle, "tpu_chip_count");
    error_msg = dlerror();
    if (error_msg != NULL) goto cleanup;

    snprintf(symbol_name, sizeof(symbol_name), "%s", "tpu_pids");
    tpu_pids = dlsym(handle, "tpu_pids");
    error_msg = dlerror();
    if (error_msg != NULL) goto cleanup;

    snprintf(symbol_name, sizeof(symbol_name), "%s", "tpu_metrics");
    tpu_metrics = dlsym(handle, "tpu_metrics");
    error_msg = dlerror();
    if (error_msg != NULL) goto cleanup;

    return 0;

    cleanup:
        printf("tpu_chip_count symbol cannot be resolved with error: %s\n", error_msg);
        return 1;
}

int main() {
    if (resolve_symbols() != 0) return 1;

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
        return 1;
    }
    for (int i = 0; i < n; i ++) {
        printf("%lld %lld %lld %.2f\n", device_ids[i], memory_usage[i], total_memory[i], duty_cycle_pct[i]);
    }
    
    return 0;
}
