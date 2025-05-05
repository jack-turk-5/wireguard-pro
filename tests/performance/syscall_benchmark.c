#define _GNU_SOURCE
#include <stdio.h>
#include <fcntl.h>
#include <unistd.h>
#include <time.h>

#define ITERS 10000000

double elapsed(struct timespec a, struct timespec b) {
    return (b.tv_sec - a.tv_sec) + (b.tv_nsec - a.tv_nsec) * 1e-9;
}

int main() {
    int fd = open("/dev/null", O_WRONLY);
    if (fd < 0) { perror("open"); return 1; }

    struct timespec t0, t1;
    const char buf[1] = {0};

    clock_gettime(CLOCK_MONOTONIC, &t0);
    for (int i = 0; i < ITERS; i++) {
        write(fd, buf, 1);
    }
    clock_gettime(CLOCK_MONOTONIC, &t1);

    double secs = elapsed(t0, t1);
    printf("write()/iter: %.2f ns\n", (secs / ITERS) * 1e9);
    close(fd);
    return 0;
}