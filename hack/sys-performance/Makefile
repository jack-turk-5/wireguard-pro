CC=gcc
CLIBS=-lcrypto -lssl -pthread

all: benchmark-copy benchmark-crypto benchmark-ctxswitch benchmark-malloc benchmark-syscall

benchmark-copy:
	$(CC) copy_benchmark.c -o copy_benchmark $(CLIBS)
	./copy_benchmark

benchmark-crypto:
	$(CC) crypto_benchmark.c -o crypto_benchmark $(CLIBS)
	./crypto_benchmark

benchmark-ctxswitch:
	$(CC) ctxswitch_benchmark.c -o ctxswitch_benchmark $(CLIBS)
	./ctxswitch_benchmark

benchmark-malloc:
	$(CC) malloc_benchmark.c -o malloc_benchmark $(CLIBS)
	./malloc_benchmark

benchmark-syscall:
	$(CC) syscall_benchmark.c -o syscall_benchmark $(CLIBS)
	./syscall_benchmark