#!/bin/env python3
import os
from urllib.request import urlopen

# region: conf
NAME = 'wg0'
NET = '192.168.223'
PORT = 16783
DNS = ['114.114.114.114', '8.8.8.8']
MTU = 1420
POST_UP = [
    f'iptables -A FORWARD -i {NAME} -j ACCEPT',
    f'iptables -A FORWARD -o {NAME} -j ACCEPT',
    'iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE'
]
POST_DOWN = [
    f'iptables -D FORWARD -i {NAME} -j ACCEPT',
    f'iptables -D FORWARD -o {NAME} -j ACCEPT',
    'iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE'
]
CONF_PATH = f"{NAME}.conf"
# endregion: conf

INTERFACE_KEY = 'Interface'
PEER_KEY = 'Peer'


def read_conf(lines):
    conf = {}
    for line in lines:
        i = line.find('#')
        if i >= 0:
            line = line[:i]
        line = line.strip()
        if not line:
            continue
        if line[:1] == '[':
            sec = line[1:-1]
            sec = sec.strip()
            if sec in conf:
                if not isinstance(conf[sec], list):
                    conf[sec] = [conf[sec]]
                conf[sec].append({})
            else:
                conf[sec] = {}
        else:
            i = line.find('=')
            k = line[:i].strip()
            cur = conf[sec]
            if isinstance(conf[sec], list):
                cur = cur[-1]
            cur[k] = line[i + 1:].strip()
        pass
    return conf


def conf2str(conf: dict):
    data = ''
    for kk, vv in conf.items():
        if not isinstance(vv, list):
            vv = [vv]
        for sec in vv:
            data += f'[{kk}]\n'
            data += ''.join(f"{k} = {v}\n" for k, v in sec.items())
            data += '\n'
    return data


def get_pub_ip():
    return urlopen('http://ifconfig.me').read().decode().strip()


def exec_cmd(cmd: str):
    with os.popen(cmd) as r:
        return r.read().strip()


def gen_pri_key():
    return exec_cmd('wg genkey')


def get_pub_key(pri_key: str):
    return exec_cmd(f'echo {pri_key} | wg pubkey')


def load_server_conf(conf_path: str):
    if not os.path.isfile(conf_path):
        return {}
    with open(conf_path) as f:
        return read_conf(f.readlines())


def init_server_conf(
    s_pri_key: str,
    net: str,
    port: int,
    post_up: list = None,
    post_down: list = None,
    dns: list = None,
    mtu: int = None,
):
    server_addr = f"{net}.254/24"
    rst = {
        'PrivateKey': s_pri_key,
        'Address': server_addr,
        'ListenPort': str(port),
    }
    if post_up:
        rst['PostUp'] = '; '.join(post_up)
    if post_down:
        rst['PostDown'] = '; '.join(post_down)
    if dns:
        rst['DNS'] = ', '.join(dns)
    if mtu:
        rst['MTU'] = str(mtu)
    return rst


def create_peer(
    s_pub_key: str,
    num: int,
    net: str,
    server_addr: str,
    dns: list = None,
    mtu: int = None,
):
    c_pri_key = gen_pri_key()
    c_pub_key = get_pub_key(c_pri_key)
    peer_addr = f'{net}.{num}/24'
    vpn_net = f'{net}.0/24'

    server_peer = {
        'PublicKey': c_pub_key,
        'AllowedIPs': peer_addr.split('/')[0] + '/32',
    }
    client_peer = {
        INTERFACE_KEY: {
            'PrivateKey': c_pri_key,
            'Address': peer_addr,
        },
        PEER_KEY: {
            'PublicKey': s_pub_key,
            'Endpoint': server_addr,
            'AllowedIPs': vpn_net,
            'PersistentKeepalive': '25',
        }
    }
    if dns:
        client_peer[INTERFACE_KEY]['DNS'] = ', '.join(dns)
    if mtu:
        client_peer[INTERFACE_KEY]['MTU'] = str(mtu)
    return server_peer, client_peer


def get_peer_num(conf: dict):
    if PEER_KEY not in conf:
        return 1
    peers = conf[PEER_KEY]
    if not isinstance(peers, list):
        peers = [peers]
    num = 1
    for peer in peers:
        addr: str = peer['AllowedIPs']
        num = max(int(addr.split('.')[-1].split('/')[0]), num)
    return num + 1


def is_exist_cmd(cmd_list: list):
    if 'not found' in exec_cmd('type'):
        return ['type: command not found']
    return [
        x for x in exec_cmd('type ' + ' '.join(cmd_list)).split('\n')
        if 'not found' in x
    ]


def check_env():
    rst = is_exist_cmd(['wg', 'wg-quick', 'resolvconf', 'ip', 'iptables'])
    if rst:
        return '\n'.join(rst)
    if '1' != exec_cmd('sysctl -n net.ipv4.ip_forward'):
        return 'net.ipv4.ip_forward not turned on'
    return ''


if __name__ == '__main__':
    rst = check_env()
    if rst:
        print(rst)
        exit(1)
    conf = load_server_conf(CONF_PATH)
    if INTERFACE_KEY not in conf:
        s_pri_key = gen_pri_key()
        conf[INTERFACE_KEY] = init_server_conf(
            s_pri_key=s_pri_key,
            net=NET,
            port=PORT,
            post_up=POST_UP,
            post_down=POST_DOWN,
            dns=DNS,
            mtu=MTU,
        )
    else:
        s_pri_key = conf[INTERFACE_KEY]['PrivateKey']
    s_pub_key = get_pub_key(s_pri_key)
    num = get_peer_num(conf)
    pub_ip = get_pub_ip()
    sp, cp = create_peer(
        s_pub_key=s_pub_key,
        num=num,
        net=NET,
        server_addr=f"{pub_ip}:{PORT}",
        dns=DNS,
        mtu=MTU,
    )
    if PEER_KEY not in conf:
        conf[PEER_KEY] = []
    if not isinstance(conf[PEER_KEY], list):
        conf[PEER_KEY] = [conf[PEER_KEY]]
    conf[PEER_KEY].append(sp)

    with open(CONF_PATH, 'w') as f:
        f.write(conf2str(conf))
    if not os.path.exists(NAME):
        os.makedirs(NAME)
    with open(f"{NAME}/c{num}.conf", 'w') as f:
        f.write(conf2str(cp))

# systemctl start wg-quick@wg0 && systemctl status wg-quick@wg0
# systemctl status wg-quick@wg0
# journalctl -r -u wg-quick@wg0
# netsh advfirewall firewall add rule name= "All ICMP V4" protocol=icmpv4:any,any dir=in action=allow
