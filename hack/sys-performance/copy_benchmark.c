#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <time.h>

#define BUF_SIZE (100 * 1024 * 1024)  // 100MiB
#define ITERS 10

double elapsed(struct timespec a, struct timespec b) {
    return (b.tv_sec - a.tv_sec) + (b.tv_nsec - a.tv_nsec) * 1e-9;
}

int main() {
    void *src = malloc(BUF_SIZE), *dst = malloc(BUF_SIZE);
    if (!src || !dst) { perror("alloc"); return 1; }

    // touch pages
    memset(src, 0xAA, BUF_SIZE);
    memset(dst, 0, BUF_SIZE);

    struct timespec t0, t1;
    clock_gettime(CLOCK_MONOTONIC, &t0);
    for (int i = 0; i < ITERS; i++) {
        memcpy(dst, src, BUF_SIZE);
    }
    clock_gettime(CLOCK_MONOTONIC, &t1);

    double secs = elapsed(t0, t1) / ITERS;
    double mib = BUF_SIZE / (1024.0 * 1024.0);
    printf("memcpy: %.2f MiB/s\n", mib / secs);

    free(src); free(dst);
    return 0;
}
