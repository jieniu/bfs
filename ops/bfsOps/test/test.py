#!/usr/bin/env python
# -*- coding: utf-8 -*-

import urllib2
import json
import sys

def groups():
	url = 'http://127.0.0.1:9000/bfsops/groups'
	#url = 'http://127.0.0.1:9000/volumes'
	value = {"ips":"10.10.191.8,10.10.191.9","copys":"2","rack":"1"}

	jdata = json.dumps(value)
	req = urllib2.Request(url, jdata, headers = {"Content-type":"application/json"})
	response = urllib2.urlopen(req)
	return response.read()

def get_groups():
	url = 'http://127.0.0.1:9000/bfsops/groups'
	response = urllib2.urlopen(url)		
	return response.read()


def init_post():
	url = 'http://127.0.0.1:9000/bfsops/initialization'
	value = {"ips":"10.10.191.8,10.10.191.9","dirs":"/data2/sd","size":"100G"}

	jdata = json.dumps(value)
	req = urllib2.Request(url, jdata, headers = {"Content-type":"application/json"})
	response = urllib2.urlopen(req)
	return response.read()

def init_get():
	url = 'http://127.0.0.1:9000/bfsops/initialization'
	response = urllib2.urlopen(url)
	return response.read()

def init_volumes_post():
	url = 'http://127.0.0.1:9000/bfsops/volumes'
	value = {"groups":"1"}
	jdata = json.dumps(value)
	req = urllib2.Request(url, jdata, headers = {"Content-type":"application/json"})
	response = urllib2.urlopen(req)
	return response.read()

resp = ''

if len(sys.argv) != 3:
	print "usage: python %s init get/post" % sys.argv[1]
	sys.exit(0)

if sys.argv[1] == 'init':
	if sys.argv[2] == 'post':
		resp = init_post()
	elif sys.argv[2] == 'get':
		resp = init_get()	
elif sys.argv[1] == 'groups':
	if sys.argv[2] == 'post':
		resp = groups()
	elif sys.argv[2] == 'get':
		resp = get_groups()
elif sys.argv[1] == "volumes":
	if sys.argv[2] == "post":
		resp = init_volumes_post()
else: 
	print "wrong args"
	sys.exit(0)

print resp
