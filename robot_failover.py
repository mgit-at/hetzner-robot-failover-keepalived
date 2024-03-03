#!/usr/bin/env python3
# (c) 2018 Maximilian Siegl

import sys
import json
import os
import requests
from multiprocessing import Process
from bunch import bunchify
from base64 import b64encode

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


def main(arg_vrouter, arg_type, arg_name, arg_endstate):
    with open(CONFIG_PATH, "r") as config_file:
        config = bunchify(json.load(config_file))

    # arg_vrouter is server id whose failover we should switch
    # we take all fallback ips belonging to arg_vrouter and switch them for ours (config.this_server_id)

    header = None

    print("Perform action for transition on %s router id with own id %s to %s state" % (arg_vrouter, config.this_router_id, arg_endstate))

    our = config.main_ips[str(config.this_router_id)]

    for ip in config.floating_ips:
        if ip.router == arg_vrouter:
            addr = ip.ip
            # this is the floating ip api request
            url = config.url_floating.format(addr)

            our = None

            if ':' in addr:
                addr += config.ipv6_suffix
                our = our.ipv6
            else
                our = our.ipv4

            payload_floating = None

            owner = ip.owner if 'owner' in ip else ip.router

            if not 'use_vlan_ips' in config or not config.use_vlan_ips:
                auth = config.robot_auth if 'robot_auth' in config and config.robot_auth else config.robot_auths[str(owner)]
                # this sets the headers for making a request to robot api
                # which is not required when only switching vlan ips
                header = {
                    "Content-Type": "application/json",
                    "Authorization": "Basic " + b64encode(bytes(auth, 'utf-8')).decode('utf-8')
                }


            # we only need to specify the address if switching to *another* target
            # if we switch to ourselves we need to send a delete request
            if owner != config.this_router_id:
                payload_floating = "active_server_ip={}".format(our)

            Process(target=change_request, args=(arg_endstate, url, header, payload_floating,
                                             config.iproute2_bin, addr, config.interface)).start()

if __name__ == "__main__":
    main(arg_vrouter=int(sys.argv[1]), arg_type=sys.argv[2], arg_name=sys.argv[3], arg_endstate=sys.argv[4])
