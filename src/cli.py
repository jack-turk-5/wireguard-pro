import asyncio


async def run_command(command, stdin_input=None):
    """Helper to run a shell command asynchronously."""
    process = await asyncio.create_subprocess_shell(
        command,
        stdin=asyncio.subprocess.PIPE if stdin_input else None,
        stdout=asyncio.subprocess.PIPE,
        stderr=asyncio.subprocess.PIPE,
    )
    stdout, stderr = await process.communicate(
        input=stdin_input.encode() if stdin_input else None
    )

    if process.returncode != 0:
        raise RuntimeError(
            f"Command '{command}' failed with stderr: {stderr.decode().strip()}"
        )

    return stdout.decode().strip()
