#!/usr/bin/env python3
from getpass import getpass
from subprocess import run, DEVNULL
from sys import exit, stderr

def create(name):
    # If secret exists, skip instead of re-prompting
    if run(["podman", "secret", "exists", name],
           stdout=DEVNULL, stderr=DEVNULL).returncode == 0:
        print(f"✓ Secret '{name}' already exists, skipping.")
        return

    # Else prompt & create it
    val = getpass(f"Enter {name.replace('-',' ')}: ")
    res = run(
        ["podman", "secret", "create", name, "-"],
        input=val.encode(),
    )
    if res.returncode:
        print(f"Error creating '{name}'", file=stderr)
        exit(res.returncode)
    print(f"✓ Secret '{name}' created.")

if __name__ == "__main__":
    create("admin-user")
    create("admin-pass")
