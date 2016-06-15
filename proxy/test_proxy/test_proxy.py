#!/usr/bin/env python
# -*- coding: utf-8 -*-

import urllib2
import sys

def upload_file(path):
	filename = path[path.rfind("/")+1:]
	url = "http://localhost:2232/xfs/test/%s" % filename
	f = open(path)			
	data = f.read()
	f.close()
	request = urllib2.Request(url, data, headers = {"Content-type":"img/png"})		
	request.get_method = lambda: 'PUT'
	response = urllib2.urlopen(request)
	return response.read()
		

def download_file(filename):
	url = "http://localhost:2232/xfs/test/%s" % filename
	request = urllib2.Request(url)		
	request.get_method = lambda: 'GET'
	response = urllib2.urlopen(request)
	return response.read()
def delete_file(filename):
	url = "http://localhost:2232/xfs/test/%s" % filename
	request = urllib2.Request(url)		
	request.get_method = lambda: 'DELETE'
	response = urllib2.urlopen(request)
	return response.read()


if __name__ != "__main__":
	sys.exit()

if len(sys.argv) < 2:
	print "usage: python ./%s upload" % sys.argv[0]
	sys.exit(0)

if sys.argv[1] == "upload":
	if len(sys.argv) != 3:
		print "usage: python ./%s upload filename" % sys.argv[0]
		sys.exit(0)
	filename = sys.argv[2]
	resp = upload_file(filename)
elif sys.argv[1] == "download":
	if len(sys.argv) != 3:
		print "usage: python ./%s download filename" % sys.argv[0]
		sys.exit(0)
	filename = sys.argv[2]
	resp = download_file(filename)
elif sys.argv[1] == "delete":
	if len(sys.argv) != 3:
		print "usage: python ./%s delete filename" % sys.argv[0]
		sys.exit(0)
	filename = sys.argv[2]
	resp = delete_file(filename)

else:
	print "parameter error"
	sys.exit()

print resp
