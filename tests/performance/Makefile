CC=gcc
CFLAGS=-lcrypto -pthread

all: benchmark-copy benchmark-crypto benchmark-ctxswitch benchmark-malloc benchmark-syscall

benchmark-copy:
	$(CC) $(CFLAGS) copy_benchmark.c -o copy_benchmark
	./copy_benchmark

benchmark-crypto:
	$(CC) $(CFLAGS) crypto_benchmark.c -o crypto_benchmark
	./crypto_benchmark

benchmark-ctxswitch:
	$(CC) $(CFLAGS) ctxswitch_benchmark.c -o ctxswitch_benchmark
	./ctxswitch_benchmark

benchmark-malloc:
	$(CC) $(CFLAGS) malloc_benchmark.c -o malloc_benchmark
	./malloc_benchmark

benchmark-syscall:
	$(CC) $(CFLAGS) syscall_benchmark.c -o syscall_benchmark
	./syscall_benchmark