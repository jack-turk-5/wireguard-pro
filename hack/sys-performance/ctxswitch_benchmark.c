#include <stdio.h>
#include <pthread.h>
#include <unistd.h>
#include <time.h>

#define ITERS 1000000

int p1[2], p2[2];

void *thread_fn(void *arg) {
    char buf;
    for (int i = 0; i < ITERS; i++) {
        read(p1[0], &buf, 1);
        write(p2[1], &buf, 1);
    }
    return NULL;
}

double elapsed(struct timespec a, struct timespec b) {
    return (b.tv_sec - a.tv_sec) + (b.tv_nsec - a.tv_nsec) * 1e-9;
}

int main() {
    pipe(p1); pipe(p2);
    pthread_t thr;
    pthread_create(&thr, NULL, thread_fn, NULL);

    struct timespec t0, t1;
    char buf = 0;
    clock_gettime(CLOCK_MONOTONIC, &t0);
    for (int i = 0; i < ITERS; i++) {
        write(p1[1], &buf, 1);
        read(p2[0], &buf, 1);
    }
    clock_gettime(CLOCK_MONOTONIC, &t1);

    double secs = elapsed(t0, t1);
    printf("context‑switch roundtrip: %.2f µs\n", (secs / ITERS) * 1e6);

    pthread_join(thr, NULL);
    return 0;
}