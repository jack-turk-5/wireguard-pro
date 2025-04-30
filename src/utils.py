from subprocess import check_output, run

def generate_keypair():
    private_key = check_output(["wg", "genkey"]).decode().strip()
    res = run(
        args=["wg", "pubkey"],
        input=private_key,
        capture_output=True,
        check=True,
        text=True
    )
    public_key = res.stdout.strip()
    return private_key, public_key

def next_available_ip():
    ipv4 = "10.8.0.2"  # Simple static IP now, can be dynamic later
    ipv6 = "fd86:ea04:1111::100"
    return ipv4, ipv6

def append_peer_to_wgconf(public_key, ipv4, ipv6):
    with open("/etc/wireguard/wg0.conf", "a") as f:
        f.write(f"""
[Peer]
PublicKey = {public_key}
AllowedIPs = {ipv4}/32, {ipv6}/128
""")

def reload_wireguard():
    run(["wg", "syncconf", "wg0", "/etc/wireguard/wg0.conf"])
