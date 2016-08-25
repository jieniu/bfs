package redis_c

import (
	"encoding/json"
	"strconv"
	"testing"
	"xfs/libs/errors"
	"xfs/libs/meta"
)

func TestExist(t *testing.T) {

	// 需要保证redis在运行
	rc, _ := NewRedisClient(20, 10, "127.0.0.1:8888")
	defer rc.Close()

	c := rc.pool.Get()
	defer c.Close()
	c.Do("del", "xxxooo")

	var (
		exists bool
		err    error
	)

	exists, err = rc.existsHashDir("/xxxooo")
	if err != errors.ErrParam {
		t.Errorf("test exists error")
	}
	exists, err = rc.existsHashDir("xxxooo/")
	if err != errors.ErrParam {
		t.Errorf("test exists error")
	}

	// put xxxooo
	c.Do("hdel", "/aaa/bbb/", "ccc/")
	c.Do("hset", "/aaa/bbb/", "ccc/", 0)
	exists, err = rc.existsHashDir("/aaa/bbb/ccc/")
	if exists != true || err != nil {
		t.Errorf("test exists error, exists %b %v", exists, err)
	}
	exists, err = rc.existsHashDir("/aaa/bbb/ddd/")
	if exists != false || err != nil {
		t.Errorf("test exists error, exists %b %v", exists, err)
	}
	c.Do("hdel", "/", "ccc/")
	c.Do("hset", "/", "ccc/", 0)
	exists, err = rc.existsHashDir("/ccc/")
	if exists != true || err != nil {
		t.Errorf("test exists error, exists %b %v", exists, err)
	}
}

func TestPutDir(t *testing.T) {
	var (
		data      interface{}
		err       error
		data_byte []byte
		ok        bool
		ret       int
		times     int
	)

	// 需要保证redis在运行
	rc, _ := NewRedisClient(20, 10, "127.0.0.1:8888")
	defer rc.Close()

	c := rc.pool.Get()
	defer c.Close()

	c.Do("hdel", "/aa/bb/cc/", "d")
	c.Do("hdel", "/aa/bb/", "cc/")
	c.Do("hdel", "/aa/", "bb/")
	c.Do("hdel", "/", "aa/")

	times, err = rc.putDir("test", "/aa/bb/cc/d", 1024)
	if times != 4 {
		t.Errorf("putdir error, times=%d", times)
	}
	if err != nil {
		t.Errorf("rc putDir failed %v", err)
	}
	data, err = c.Do("hget", "/aa/bb/cc/", "d")
	if err != nil {
		t.Errorf("hget error")
	}
	if data_byte, ok = data.([]byte); ok != true {
		t.Errorf("convert error")
	}
	if ret, err = strconv.Atoi(string(data_byte)); ret != 1024 {
		t.Errorf("convert error")
	}

	data, err = c.Do("hget", "/aa/bb/", "cc/")
	if err != nil {
		t.Errorf("hget error")
	}
	if data_byte, ok = data.([]byte); ok != true {
		t.Errorf("convert error")
	}
	if ret, err = strconv.Atoi(string(data_byte)); ret != 0 {
		t.Errorf("convert error")
	}

	data, err = c.Do("hget", "/aa/", "bb/")
	if err != nil {
		t.Errorf("hget error")
	}
	if data_byte, ok = data.([]byte); ok != true {
		t.Errorf("convert error")
	}
	if ret, err = strconv.Atoi(string(data_byte)); ret != 0 {
		t.Errorf("convert error")
	}

	data, err = c.Do("hget", "/", "aa/")
	if err != nil {
		t.Errorf("hget error")
	}
	if data_byte, ok = data.([]byte); ok != true {
		t.Errorf("convert error")
	}
	if ret, err = strconv.Atoi(string(data_byte)); ret != 0 {
		t.Errorf("convert error")
	}

	c.Do("hdel", "/aa/bb/cc/", "f")
	times, err = rc.putDir("test", "/aa/bb/cc/f", 1024)
	if times != 1 {
		t.Errorf("put dir error, times=%d", times)
	}
}

func TestDelFile(t *testing.T) {
	rc, _ := NewRedisClient(20, 10, "127.0.0.1:8888")
	defer rc.Close()

	c := rc.pool.Get()
	defer c.Close()

	var ex interface{}
	var err error
	var value int64
	var ok bool

	c.Do("set", "filename", "test")
	c.Do("hset", "/", "filename", 1024)

	err = rc.delFile("0", "filename")
	if err != nil {
		t.Errorf("delFile faile")
	}
	if ex, err = c.Do("hexists", "/", "filename"); err != nil {
		t.Errorf("redis error")
	}
	if value, ok = ex.(int64); ok != true {
		t.Errorf("conver error")
	}
	if value == 1 {
		t.Errorf("key / value filename is still exists")
	}

	c.Do("set", "/filename", "test")
	c.Do("hset", "/", "filename", 1024)

	err = rc.delFile("0", "/filename")
	if err != nil {
		t.Errorf("delFile failed")
	}
	if ex, err = c.Do("hexists", "/", "filename"); err != nil {
		t.Errorf("redis error")
	}
	if value, ok = ex.(int64); ok != true {
		t.Errorf("conver error")
	}
	if value == 1 {
		t.Errorf("key / value filename is still exists")
	}
	if ex, err = c.Do("exists", "/filename"); err != nil {
		t.Errorf("redis error")
	}
	if value, ok = ex.(int64); ok != true {
		t.Errorf("conver error")
	}
	if value == 1 {
		t.Errorf("key /filename still exists")
	}
}

func TestPutFile(t *testing.T) {
	rc, _ := NewRedisClient(20, 10, "127.0.0.1:8888")
	defer rc.Close()

	c := rc.pool.Get()
	defer c.Close()

	var (
		data  interface{}
		bytes []byte
	)

	// test exists
	var file = meta.File{}
	c.Do("set", "/myfile", 1234)
	file.Filename = "/myfile"
	err := rc.putFile("test", &file)
	if err != errors.ErrNeedleExist {
		t.Errorf("key \"/myfile\" is exists but result is not")
	}
	c.Do("del", "/myfile")
	// test not exists
	c.Do("del", "other")
	file.Filename = "other"
	file.Filesize = 1234
	err = rc.putFile("test", &file)
	if err != nil {
		t.Errorf("put file failed: %v", err)
	}
	bytes, _ = data.([]byte)
	data, err = c.Do("get", "other")
	json.Unmarshal(bytes, &file)
	if file.Filename != "other" || file.Filesize != 1234 {
		t.Errorf("filename=%s, filesize=%s", file.Filename, file.Filesize)
	}
}

func TestDelDirSubitem(t *testing.T) {

	rc, _ := NewRedisClient(20, 10, "127.0.0.1:8888")
	defer rc.Close()

	c := rc.pool.Get()
	defer c.Close()

	num, err := rc.DelDirSubitem("test", "abc", "123")
	if err != errors.ErrParam {
		t.Errorf("want ErrParam but not")
	}
	num, err = rc.DelDirSubitem("test", "/abc", "")
	if err != errors.ErrParam {
		t.Errorf("want ErrParam but not")
	}

	// test exists
	c.Do("hdel", "/mydirtest", "mydirtest")
	c.Do("hset", "/mydirtest", "mydirtest", "test")
	num, err = rc.DelDirSubitem("test", "/mydirtest", "mydirtest")
	if num != 1 || err != nil {
		t.Errorf("del dir sub item failed, num=%d, err=%v", num, err)
	}
	// test not exists
	num, err = rc.DelDirSubitem("test", "/mydirtest", "mydirtest")
	if num != 0 || err != nil {
		t.Errorf("del dir sub item failed, num=%d, err=%v", num, err)
	}

}

func TestDeleteHashKeys(t *testing.T) {
	rc, _ := NewRedisClient(20, 10, "127.0.0.1:8888")
	defer rc.Close()

	c := rc.pool.Get()
	defer c.Close()

	// test exists
	c.Do("hset", "mytest", "mytest", "mytest")

	var keys = make(map[string]string)
	keys["mytest"] = "mytest"
	ret, err := rc.deleteHashKeys(keys)
	if err != nil {
		t.Errorf("delete hash keys failed: %s", err)
	}
	if ret != 1 {
		t.Errorf("delete hash keys failed: ret = %d", ret)
	}
	// test not exists
	ret, err = rc.deleteHashKeys(keys)
	if err != nil {
		t.Errorf("delete hash keys failed: %s", err)
	}
	if ret != 0 {
		t.Errorf("delete hash keys failed: ret = %d", ret)
	}
}

func TestGetDirInfo(t *testing.T) {

	rc, _ := NewRedisClient(20, 10, "127.0.0.1:8888")
	defer rc.Close()

	c := rc.pool.Get()
	defer c.Close()

	// test not exists
	_, err := rc.GetDirInfo("test", "/notexists")
	if err != errors.ErrDirNotExist {
		t.Errorf("test dir not exist failed.")
	}

	// exists
	c.Do("hset", "/atestdir/", "file1", 123)
	c.Do("hset", "/atestdir/", "file2", 456)
	c.Do("hset", "/atestdir/", "subdir1/", 0)
	c.Do("hset", "/atestdir/", "subdir2/", 0)

	var dirinfo = meta.DirInfo{}
	dirinfo, err = rc.GetDirInfo("test", "/atestdir/")
	if err != nil {
		t.Errorf("get dir /atestdir info failed, %v", err)
	}
	if dirinfo.Files[0] != "file1" || dirinfo.Files[1] != "file2" {
		t.Errorf("get dir /atestdir info failed, %v", err)
	}
	if dirinfo.SubDirs[0] != "subdir1/" || dirinfo.SubDirs[1] != "subdir2/" {
		t.Errorf("get dir /atestdir info failed, %v", err)
	}
	if dirinfo.Dir != "/atestdir/" {
		t.Errorf("get dir /atestdir info failed, %v", err)
	}
}
