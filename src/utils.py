import asyncio
from db import get_all_peers
from src.config import get_config
from aiofiles import open

WG_PATH = "/etc/wireguard/wg0.conf"


async def _run_command(command, stdin_input=None):
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


async def generate_keypair():
    """
    Generate a private/public keypair asynchronously.
    """
    private_key = await _run_command("wg genkey")
    public_key = await _run_command("wg pubkey", stdin_input=private_key)
    return private_key, public_key


async def get_server_pubkey():
    """
    Derive server public key from stored private key asynchronously.
    """
    async with open("/etc/wireguard/privatekey") as f:
        priv = (await f.read()).strip()
    return await _run_command("wg pubkey", stdin_input=priv)


async def next_available_ip():
    """
    Allocate the next free IPv4/IPv6 addresses asynchronously.
    """
    config = await get_config()
    peers = get_all_peers()
    used_v4 = {p["ipv4_address"] for p in peers}
    used_v6 = {p["ipv6_address"] for p in peers}

    ipv4_base = config.wg_ipv4_base_addr[:-1]
    ipv6_base = config.wg_ipv6_base_addr[:-1]

    ipv4 = ""
    for i in range(2, 255):
        candidate = f"{ipv4_base}{i}"
        if candidate not in used_v4:
            ipv4 = candidate
            break
    else:
        raise RuntimeError(f"No free IPv4 addresses left in {ipv4_base}0/24")

    ipv6 = ""
    for suffix in range(0x100, 0x10000):
        candidate6 = f"{ipv6_base}{suffix:x}"
        if candidate6 not in used_v6:
            ipv6 = candidate6
            break
    else:
        raise RuntimeError(f"No free IPv6 addresses left in {ipv6_base}/64")

    return ipv4, ipv6


async def append_peer_to_wgconf(public_key, ipv4, ipv6):
    """
    Append a peer to the wg0.conf file asynchronously.
    """
    config = await get_config()
    allowed_ips = config.wg_allowed_ips
    async with open(WG_PATH, "a") as f:
        lines = [
            "",
            "[Peer]",
            f"PublicKey = {public_key}",
            f"AllowedIPs = {allowed_ips}",
        ]
        await f.write("\n".join(lines) + "\n")


async def reload_wireguard():
    """
    Reload peers dynamically (strip + syncconf) asynchronously.
    """
    stripped_config = await _run_command(f"wg-quick strip {WG_PATH}")

    async with open("/tmp/wg0.peers.conf", "w") as f:
        await f.write(stripped_config)

    await _run_command("wg syncconf wg0 /tmp/wg0.peers.conf")


async def remake_peers_file():
    """
    Rebuilds the WireGuard config from the database and reloads the interface asynchronously.
    """
    # 1) read existing file up to—but not including—the first [Peer]
    async with open(WG_PATH, "r") as f:
        content = await f.read()
    lines = content.splitlines()
    interface_section = []
    for line in lines:
        if line.strip() == "[Peer]":
            break
        interface_section.append(line)

    # 2) overwrite disk config with just the interface
    async with open(WG_PATH, "w") as f:
        await f.write("\n".join(interface_section))

    # 3) re-append every peer from the DB
    for p in get_all_peers():
        await append_peer_to_wgconf(
            p["public_key"], p["ipv4_address"], p["ipv6_address"]
        )

    # 4) push into the running interface
    await reload_wireguard()

