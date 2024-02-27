#!/usr/bin/env python3
# (c) 2018 Maximilian Siegl

import sys
import json
import os
import requests
from multiprocessing import Process
from bunch import bunchify

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


def change_request(endstate, url, header, payload, ip_bin_path, floating_ip, interface):
    if endstate == "BACKUP":
        del_ip(ip_bin_path, floating_ip, interface)
    elif endstate == "FAULT":
        del_ip(ip_bin_path, floating_ip, interface)

    elif endstate == "MASTER":
        add_ip(ip_bin_path, floating_ip, interface)
        if header:
            if payload:
                print("Post request to: " + url)
                print("Header: " + str(header))
                print("Data: " + str(payload))
                r = requests.post(url, data=payload, headers=header)
                print("Response:")
                print(r.status_code, r.reason)
                print(r.text)
            else:
                print("Delete request to: " + url)
                print("Header: " + str(header))
                r = requests.delete(url, headers=header)
                print("Response:")
                print(r.status_code, r.reason)
                print(r.text)#
    else:
        print("Error: Endstate not defined!")


def main(arg_type, arg_name, arg_endstate):
    with open(CONFIG_PATH, "r") as config_file:
        config = bunchify(json.load(config_file))

    # this is server id whose failover we should switch
    # we take all fallback ips belonging to this server and switch them for ours
    virtual_router_id = 2

    header = None

    if not 'use_private_ips' in config or not config.use_private_ips:
        header = {
            "Content-Type": "application/json",
            "Authorization": "Basic " + (config.robot_password if 'robot_password' in config else config.robot_passwords[str(virtual_router_id)])
        }

    print("Perform action for transition on %s router id with own id %s to %s state" % (virtual_router_id, config.this_router_id, arg_endstate))

    our_v4 = None
    our_v6 = None
    for ip in config.floating_ips:
        if ip.router == config.this_router_id:
            if ':' in ip.ip:
                our_v6 = ip.ip
            else:
                our_v4 = ip.ip

    for ip in config.floating_ips:
        if ip.router == virtual_router_id:
            addr = ip.ip
            # this is the floating ip api request
            url = config.url_floating.format(addr)

            our = our_v4

            if ':' in addr:
                addr += config.ipv6_suffix
                our = our_v6

            payload_floating = None

            # we only need to specify the address if switching to *another* target
            # if we switch to ourselves we need to send a delete request
            if virtual_router_id != config.this_router_id:
                payload_floating = "active_server_ip={}".format(our)

            Process(target=change_request, args=(arg_endstate, url, header, payload_floating,
                                             config.iproute2_bin, ip.ip , config.interface)).start()

if __name__ == "__main__":
    main(arg_type=sys.argv[1], arg_name=sys.argv[2], arg_endstate=sys.argv[3])
