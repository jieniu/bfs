#!/usr/bin/python
from kazoo.client import KazooClient
import config
from kazoo.protocol.paths import join
import json
import httplib2


max_block_size = 4294967295
padding_size = 8


#class StoreInfo:

# read store file get all stores
stores = {}
f = open("store.txt")
for l in f:
    l = l.strip()
    store = {}
    store["ip"] = l
    store["status"] = 0 # 0=close;1=up;2=abnormal
    stores[l] = store

# get zookeeper rack
zk_stores = []
store_ip = {}
zk_client = KazooClient(hosts=config.zk_hosts)
zk_client.start()
children = zk_client.get_children("/rack")
for child in children:
    rack_name = child.encode('utf-8')
    path1 = join('/rack', rack_name)
    children1 = zk_client.get_children(path1)
    for child1 in children1:
        store_id = child1.encode('utf-8')
        path2 = join(path1, store_id)
        data, stat = zk_client.get(path2)
        if data:
            parsed_data = json.loads(data)
            rw_status = int(parsed_data['status'])
            rw_status_int = 3
            # only read=1;read write=2;other=3
            if rw_status == 0x80000003:
                rw_status_int = 2
            elif rw_status == 0x80000001:
                rw_status_int = 1
            ip = parsed_data['stat'].split(":")[0].encode('utf-8')
            stat_api = parsed_data['stat'].encode('utf-8')
            store_id = parsed_data['id']
            store_ip[store_id] = ip
            if ip not in stores.keys():
                store = {}
                store["ip"] = ip
                store["status"] = 1
                store["last_heart_beat"] = int(stat.last_modified)
                store["rw_status"] = rw_status_int
                store["stat_api"] = stat_api
                stores[ip] = store
            else:
                store = stores[ip]
                store["status"] = 1
                store["last_heart_beat"] = int(stat.last_modified)
                store["rw_status"] = rw_status_int
                store["stat_api"] = stat_api
                stores[ip] = store

# get zookeeper group
children = zk_client.get_children("/group")
for child in children:
    group_id = child.encode("utf-8")
    path1 = join('/group', group_id)
    children1 = zk_client.get_children(path1)
    for child1 in children1:
        if child1 in store_ip.keys():
            ip = store_ip[child1]
            if ip in stores.keys():
                store = stores[ip]
                store["group"] = group_id
                stores[ip] = store


stores2 = {}
for key, v in stores.items():
    if v["status"] == 0:
        stores2[key] = v
        continue
    stat_api = v['stat_api']
    h = httplib2.Http(".cache")
    resp, content = h.request("http://%s/info" % stat_api, "GET")
    if resp["status"] != '200':
        print "request %s failed, status,"% stat_api, resp
        continue
    parsed_data = json.loads(content)
    volumes = parsed_data["volumes"]
    size = 0
    total_size = 0
    total_delay_ns = 0
    total_process = 0
    total_needle_number = 0
    for volume in volumes:
        size += (max_block_size - volume['block']['offset'])
        total_delay_ns += volume['stats']['total_delay']
        total_process += volume['stats']['total_commands_processed']
        total_needle_number += volume['needle_number']

    size *= padding_size
    total_size = padding_size * len(volumes) * max_block_size
    delay = total_delay_ns / (total_process * 1000)

    v["size"] = size
    v["total_size"] = total_size
    v["delay_ms"] = delay
    v["total_needel_number"] = total_needle_number

    stores2[key] = v


print stores2


