CC=gcc
CFLAGS=-03 -lcrypto -pthread

all: test-copy test-crypto test-ctxswitch test-malloc test-syscall

test-copy:
	$(CC) $(CFLAGS) copy_benchmark.c -o copy_benchmark
	./copy_benchmark

test-crypto:
	$(CC) $(CFLAGS) crypto_benchmark.c -o crypto_benchmark
	./crypto_benchmark

test-ctxswitch:
	$(CC) $(CFLAGS) ctxswitch_benchmark.c -o ctxswitch_benchmark
	./ctxswitch_benchmark

test-malloc:
	$(CC) $(CFLAGS) malloc_benchmark.c -o malloc_benchmark
	./malloc_benchmark

test-syscall:
	$(CC) $(CFLAGS) syscall_benchmark.c -o syscall_benchmark
	./syscall_benchmark