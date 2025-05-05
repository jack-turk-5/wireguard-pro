// bench_alloc.c
#include <stdio.h>
#include <stdlib.h>
#include <time.h>

#define ITERS 1000000
#define ALLOC_SIZE 4096

double elapsed(struct timespec a, struct timespec b) {
    return (b.tv_sec - a.tv_sec) + (b.tv_nsec - a.tv_nsec) * 1e-9;
}

int main() {
    struct timespec t0, t1;
    void *ptr;

    clock_gettime(CLOCK_MONOTONIC, &t0);
    for (int i = 0; i < ITERS; i++) {
        ptr = malloc(ALLOC_SIZE);
        free(ptr);
    }
    clock_gettime(CLOCK_MONOTONIC, &t1);

    double secs = elapsed(t0, t1);
    printf("malloc+free: %.2f µs per op\n", (secs / ITERS) * 1e6);
    return 0;
}