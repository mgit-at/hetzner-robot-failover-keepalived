# Hetzner Robot - Failover IP and Private IP switchover - keepalived

This is a little script for switching the failover ips on robot dedicated servers. It's also possible to use a robot vlan.

I am using this script in combination with [keepalived](http://www.keepalived.org). It is tested on NixOS based Systems.

**Credits:** [r3vival](https://github.com/r3vival) | [lehuizi](https://github.com/lehuizi)  
**License:** MIT


## How to

**1. Clone the repo**
```
apt install git
git clone https://github.com/lehuizi/hcloud-failover-keepalived.git /opt/hcloud-failover
```

**2. Install requirements**  
```
apt install python3 python3-pip keepalived
pip3 install -r /opt/hcloud-failover/requirements.txt
```

**3. Copy config.json.sample to config.json**  
```
cd /opt/hcloud-failover
cp config.json.sample config.json
```

**4. Create robot passwords in Hetzner Robot**  
1. Login to Hetzner Robot
2. Server > Select a server > Admin Access
3. Enter a dedicated admin access password
4. Add <admin password> as robot password

**5. Fill in the router ids and the fallback ips**
The router IDs are up to your choosing (keepalived limits them to 255).
Add the failover IPs without their netmask (and for IPv6 without the suffix aswell).
The IPv6 Suffix is configured using ipv6_suffix

**Robot VLAN (optional)**
If you are using a robot VLAN instead of failover IPs simply enable use_vlan_ips

---

Command:  
```
python3 /path/to/robot_failover.py [virtual_router_id] [type] [name] [endstate]
```
