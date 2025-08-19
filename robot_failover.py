#!/usr/bin/env python3
# (c) 2018.2025 Maximilian Siegl
# (c) 2023-2025 Maciej Krüger

import sys
import json
import os
import requests
import syslog
import traceback
from multiprocessing import Process
from subprocess import check_output
from bunch import bunchify
from base64 import b64encode
from time import sleep

PROTO_ID = 253

CONFIG_PATH_DEV = os.path.join(os.path.abspath(
    os.path.dirname(__file__)), "config.json")

CONFIG_PATH = os.path.join(os.path.abspath(
    "/etc/robot-failover"), "config.json")

if os.path.isfile(CONFIG_PATH_DEV):
    CONFIG_PATH = CONFIG_PATH_DEV

DEBUG = sys.stdout.isatty()
FORCE_DEBUG = 'FORCE_DEBUG_FAILOVER' in os.environ


def debug_log(*args, **kwargs):
    if DEBUG:
        print(*args, **kwargs)
    elif FORCE_DEBUG:
        syslog.syslog(syslog.LOG_INFO, *args, **kwargs)


def normal_log(*args, **kwargs):
    if DEBUG:
        print(*args, **kwargs)
    else:
        syslog.syslog(syslog.LOG_INFO, *args, **kwargs)


def disable_ip(ip_bin_path, ip, interface, use_proto):
    normal_log('[%s] disabling ip %s' % (interface, ip))
    if ':' in ip:
        os.system(ip_bin_path + " -6 addr replace " + ip + "/128 dev " + interface + ( f" proto {PROTO_ID}" if use_proto else "") + " preferred_lft 0")
    else:
        os.system(ip_bin_path + " addr replace " + ip + "/32 dev " + interface + ( f" proto {PROTO_ID}" if use_proto else "") + " preferred_lft 0")


def enable_ip(ip_bin_path, ip, interface, use_proto):
    normal_log('[%s] enabling ip %s' % (interface, ip))
    if ':' in ip:
        os.system(ip_bin_path + " -6 addr replace " + ip + "/128 dev " + interface + ( f" proto {PROTO_ID}" if use_proto else "") + " preferred_lft forever")
    else:
        os.system(ip_bin_path + " addr replace " + ip + "/32 dev " + interface + ( f" proto {PROTO_ID}" if use_proto else "") + " preferred_lft forever")


def del_ip(ip_bin_path, ip, interface, use_proto):
    normal_log('[%s] removing ip %s' % (interface, ip))
    if ':' in ip:
        os.system(ip_bin_path + " -6 addr del " + ip + "/128 dev " + interface + ( f" proto {PROTO_ID}" if use_proto else ""))
    else:
        os.system(ip_bin_path + " addr del " + ip + "/32 dev " + interface + ( f" proto {PROTO_ID}" if use_proto else ""))


def add_ip(ip_bin_path, ip, interface, use_proto):
    normal_log('[%s] adding ip %s' % (interface, ip))
    if ':' in ip:
        os.system(ip_bin_path + " -6 addr add " + ip + "/128 dev " + interface + ( f" proto {PROTO_ID}" if use_proto else ""))
    else:
        os.system(ip_bin_path + " addr add " + ip + "/32 dev " + interface + ( f" proto {PROTO_ID}" if use_proto else ""))


def has_ip(ip_bin_path, ip, interface, use_proto):
    # if this command returns any output, the address exists on the interface
    # if it's deprecated it means it's not to be used, we translate that to "doesn't exist"
    if use_proto:
        out = check_output([ip_bin_path, 'a', 's', interface, 'to', ip, 'proto', str(PROTO_ID)])
    else:
        out = check_output([ip_bin_path, 'a', 's', interface, 'to', ip])
    return bool(len(out)) and not 'deprecated' in str(out)

def change_request_wrapper(*args, **kwargs):
    try:
        return change_request(*args, **kwargs)
    except Exception as e:
        normal_log("Exception in change thread:")
        normal_log(traceback.format_exc())

def change_request(endstate, url, header, target_ip, ip_bin_path, floating_ip, interface, use_proto, dummy_interface):
    log_prefix = "[%s -> %s] S " % (url, target_ip)
    if endstate == "BACKUP" or endstate == "FAULT" or endstate == "STOP":
        if dummy_interface:
            del_ip(ip_bin_path, floating_ip, interface, use_proto)
            add_ip(ip_bin_path, floating_ip, dummy_interface, use_proto)
        else:
            disable_ip(ip_bin_path, floating_ip, interface, use_proto)

    elif endstate == "MASTER":
        if dummy_interface:
            add_ip(ip_bin_path, floating_ip, interface, use_proto)
            del_ip(ip_bin_path, floating_ip, dummy_interface, use_proto)
        else:
            enable_ip(ip_bin_path, floating_ip, interface, use_proto)
        if header:
            recheck = False
            while True:
                if not has_ip(ip_bin_path, floating_ip, interface, use_proto):
                    normal_log(log_prefix + 'ip %s has vanished from interface %s, cancelling attempt to switch' % (floating_ip, interface))
                    break
                current = requests.get(url, headers=header)
                current = current.json()
                payload = None

                payload = "active_server_ip={}".format(target_ip)

                if 'failover' in current and current['failover']['status'] == 'processing':
                    normal_log(log_prefix + 'is currently processing, waiting, rechecking in 30s...')
                    sleep(30)
                    continue
                if 'failover' in current and current['failover']['active_server_ip'] == target_ip and current['failover']['status'] == 'ready':
                    normal_log(log_prefix + 'failed over as requested already, need no switch')
                    break

                normal_log(log_prefix + "Post request to: " + url)
                debug_log(log_prefix + "Header: " + str(header))
                debug_log(log_prefix + "Data: " + str(payload))
                r = requests.post(url, data=payload, headers=header)
                debug_log(log_prefix + "Response: %s %s" % (r.status_code, r.reason))
                debug_log(r.text)
                j = r.json()
                if r.status_code != 409 or j['error']['code'] != 'FAILOVER_LOCKED':
                    if r.status_code != 200:
                        normal_log(log_prefix + "Failed with: %s %s" % (r.status_code, r.reason))
                        normal_log(r.text)
                    else:
                        normal_log(log_prefix + 'done')
                    if not recheck:
                        recheck = True
                        log_prefix = "[%s -> %s] R " % (url, target_ip)
                        normal_log(log_prefix + 'rechecking in 30s...')
                        sleep(30)
                        continue
                    break
                else:
                    normal_log(log_prefix + 'trying again in 120s...')
                    sleep(120)

    else:
        normal_log(f"Error: Endstate {endstate} not defined!")


def main(arg_vrouter, arg_type, arg_name, arg_endstate):
    with open(CONFIG_PATH, "r") as config_file:
        config = bunchify(json.load(config_file))

    # arg_vrouter is server id whose failover we should switch
    # we take all fallback ips belonging to arg_vrouter and switch them for ours (config.this_server_id)

    header = None

    normal_log("Perform action for transition on %s router id with own id %s to %s state" % (arg_vrouter, config.this_router_id, arg_endstate))

    main = config.main_ips[str(config.this_router_id)]

    for ip in config.floating_ips:
        if ip.router == arg_vrouter:
            addr = ip.ip
            # this is the floating ip api request
            url = config.url_floating.format(addr)

            our = None

            if ':' in addr:
                addr += config.ipv6_suffix
                our = main.ipv6
            else:
                our = main.ipv4

            owner = ip.owner if 'owner' in ip and ip.owner else ip.router

            if not 'use_vlan_ips' in config or not config.use_vlan_ips:
                auth = config.robot_auth if 'robot_auth' in config and config.robot_auth else config.robot_auths[str(owner)]
                # this sets the headers for making a request to robot api
                # which is not required when only switching vlan ips
                header = {
                    "Content-Type": "application/x-www-form-urlencoded",
                    "Authorization": "Basic " + b64encode(bytes(auth, 'utf-8')).decode('utf-8')
                }

            # wait 30s, check if still master, then do a check-and-switch again
            # this is to prevent the jumping issue
            Process(target=change_request_wrapper, args=(arg_endstate, url, header, our,
                                             config.iproute2_bin, addr, config.interface,
                                             config.use_proto if 'use_proto' in config and config.use_proto else False,
                                             config.dummy_interface if 'dummy_interface' in config and config.dummy_interface else False)
                                         ).start()

def cli():
    main(arg_vrouter=int(sys.argv[1]), arg_type=sys.argv[2], arg_name=sys.argv[3], arg_endstate=sys.argv[4])

if __name__ == "__main__":
    cli()
