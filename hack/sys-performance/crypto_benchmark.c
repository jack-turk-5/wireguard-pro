#include <openssl/evp.h>
#include <openssl/rand.h>
#include <string.h>
#include <stdlib.h>
#include <stdio.h>
#include <time.h>

// Buffer size in bytes
#define BUF_SIZE (50 * 1024 * 1024)

static double elapsed_secs(struct timespec start, struct timespec end) {
    return (end.tv_sec - start.tv_sec) + (end.tv_nsec - start.tv_nsec) * 1e-9;
}

int main(void) {
    // Allocate plaintext, ciphertext, and decrypted buffers
    unsigned char *pt = calloc(BUF_SIZE, 1);
    unsigned char *ct = malloc(BUF_SIZE + 16);
    unsigned char *dt = malloc(BUF_SIZE);
    if (!pt || !ct || !dt) {
        perror("alloc");
        return 1;
    }

    // Initialize key and nonce
    unsigned char key[32], nonce[12];
    if (!RAND_bytes(key, sizeof(key)) || !RAND_bytes(nonce, sizeof(nonce))) {
        fprintf(stderr, "RAND_bytes failed\n");
        return 1;
    }

    EVP_CIPHER_CTX *ctx = EVP_CIPHER_CTX_new();
    if (!ctx) {
        fprintf(stderr, "EVP_CIPHER_CTX_new failed\n");
        return 1;
    }

    int outlen, tmplen;
    struct timespec t0, t1;

    // Encryption benchmark
    clock_gettime(CLOCK_MONOTONIC, &t0);
    EVP_EncryptInit_ex(ctx, EVP_chacha20_poly1305(), NULL, key, nonce);
    EVP_EncryptUpdate(ctx, ct, &outlen, pt, BUF_SIZE);
    EVP_EncryptFinal_ex(ctx, ct + outlen, &tmplen);
    outlen += tmplen;
    clock_gettime(CLOCK_MONOTONIC, &t1);

    double enc_secs = elapsed_secs(t0, t1);
    double enc_mibs = (double)BUF_SIZE / (1024.0 * 1024.0);
    printf("Encrypt: %.2f MiB/s\n", enc_mibs / enc_secs);

    // Decryption benchmark
    clock_gettime(CLOCK_MONOTONIC, &t0);
    EVP_DecryptInit_ex(ctx, EVP_chacha20_poly1305(), NULL, key, nonce);
    EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_AEAD_SET_TAG, 16, ct + outlen); // set tag :contentReference[oaicite:6]{index=6}
    EVP_DecryptUpdate(ctx, dt, &outlen, ct, outlen);
    EVP_DecryptFinal_ex(ctx, dt + outlen, &tmplen);
    clock_gettime(CLOCK_MONOTONIC, &t1);

    double dec_secs = elapsed_secs(t0, t1);
    printf("Decrypt: %.2f MiB/s\n", enc_mibs / dec_secs);

    // Cleanup
    EVP_CIPHER_CTX_free(ctx);
    free(pt); free(ct); free(dt);
    return 0;
}