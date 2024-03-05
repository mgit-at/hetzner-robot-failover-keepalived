#!/usr/bin/env python3
# (c) 2018 Maximilian Siegl

import sys
import json
import os
import requests
from multiprocessing import Process
from bunch import bunchify
from base64 import b64encode
from time import sleep

CONFIG_PATH = os.path.join(os.path.abspath(
    os.path.dirname(__file__)), "config.json")


def del_ip(ip_bin_path, ip, interface):
    if ':' in ip:
        os.system(ip_bin_path + " -6 addr del " + ip + "/128 dev " + interface)
    else:
        os.system(ip_bin_path + " addr del " + ip + "/32 dev " + interface)


def add_ip(ip_bin_path, ip, interface):
    if ':' in ip:
        os.system(ip_bin_path + " -6 addr add " + ip + "/128 dev " + interface)
    else:
        os.system(ip_bin_path + " addr add " + ip + "/32 dev " + interface)


def change_request(endstate, url, header, target_ip, ip_bin_path, floating_ip, interface):
    log_prefix = "[%s -> %s] " % (url, target_ip)
    if endstate == "BACKUP":
        del_ip(ip_bin_path, floating_ip, interface)
    elif endstate == "FAULT":
        del_ip(ip_bin_path, floating_ip, interface)

    elif endstate == "MASTER":
        add_ip(ip_bin_path, floating_ip, interface)
        if header:
            while True:
                current = requests.get(url, headers=header)
                current = current.json()
                payload = None

                # we only need to specify the address if switching to *another* target
                # if we switch to ourselves we need to send a delete request
                #if current['failover']['server_ip'] != target_ip and current['failover']['server_ipv6_net'] != target_ip:

                payload = "active_server_ip={}".format(target_ip)

                if current['failover']['active_server_ip'] == target_ip:
                    print(log_prefix + 'failed over as requested already, need no switch')
                    break
                # elif not payload and not current['failover']['active_server_ip']:
                #     print(log_prefix + 'not failed over as request already, need no switch')
                #     break

                r = None

                # if payload:
                print(log_prefix + "Post request to: " + url)
                print(log_prefix + "Header: " + str(header))
                print(log_prefix + "Data: " + str(payload))
                r = requests.post(url, data=payload, headers=header)
                # else:
                #     print(log_prefix + "Delete request to: " + url)
                #     print(log_prefix + "Header: " + str(header))
                #     r = requests.delete(url, headers=header)
                print(log_prefix + "Response:")
                print(r.status_code, r.reason)
                print(r.text)
                j = r.json()
                if r.status_code != 409 or j['error']['code'] != 'FAILOVER_LOCKED':
                    break
                else:
                    print(log_prefix + 'trying again in 120s...')
                    sleep(120)

    else:
        print("Error: Endstate not defined!")


def main(arg_vrouter, arg_type, arg_name, arg_endstate):
    with open(CONFIG_PATH, "r") as config_file:
        config = bunchify(json.load(config_file))

    # arg_vrouter is server id whose failover we should switch
    # we take all fallback ips belonging to arg_vrouter and switch them for ours (config.this_server_id)

    header = None

    print("Perform action for transition on %s router id with own id %s to %s state" % (arg_vrouter, config.this_router_id, arg_endstate))

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

            Process(target=change_request, args=(arg_endstate, url, header, our,
                                             config.iproute2_bin, addr, config.interface)).start()

if __name__ == "__main__":
    main(arg_vrouter=int(sys.argv[1]), arg_type=sys.argv[2], arg_name=sys.argv[3], arg_endstate=sys.argv[4])
