package redis_c

import (
	"encoding/json"
	redis "github.com/garyburd/redigo/redis"
	log "github.com/golang/glog"
	"strings"
	"time"
	"xfs/libs/errors"
	"xfs/libs/meta"
)

type RedisClient struct {
	pool *redis.Pool
}

func NewRedisClient(max_idle int, max_timeout int, addr string) (rc *RedisClient, err error) {
	rc = &RedisClient{}
	rc.pool = &redis.Pool{
		MaxIdle:     max_idle,
		IdleTimeout: time.Duration(max_timeout) * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", addr)
			if err != nil {
				log.Warningf("redis dial failed, %v", err)
				return nil, err
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
	return rc, nil
}

func (r *RedisClient) Close() {
	r.pool.Close()
}

func (r *RedisClient) Put(bucket string, f *meta.File, n *meta.Needle) (err error) {
	if err = r.putFile(bucket, f); err != nil {
		log.Warningf("redis putFile error, bucket: %s, filename: %s", bucket, f.Filename)
		return
	}

	var filename = f.Filename
	if strings.HasPrefix(filename, "/") == false {
		filename = "/" + filename
	}
	if _, err = r.putDir(bucket, filename, f.Filesize); err != nil {
		log.Warningf("redis putDir error, filename %s", f.Filename)
		r.delFile(bucket, f.Filename)
		return
	}

	if err = r.putNeedle(n); err != errors.ErrNeedleExist && err != nil {
		log.Warningf("insert table not match: bucket: %s  filename: %s", bucket, f.Filename)
		r.delFile(bucket, f.Filename)
	}
	return
}

func (r *RedisClient) PutInfo(bucket string, f *meta.File) (err error) {
	if err = r.putFile(bucket, f); err != nil {
		log.Warningf("redis putFile error, bucket: %s, filename: %s", bucket, f.Filename)
		return
	}

	var filename = f.Filename
	if strings.HasPrefix(filename, "/") == false {
		filename = "/" + filename
	}
	if _, err = r.putDir(bucket, filename, f.Filesize); err != nil {
		log.Warningf("redis putDir error, filename %s", f.Filename)
		r.delFile(bucket, f.Filename)
	}
	return
}

func (r *RedisClient) putFile(bucket string, f *meta.File) (err error) {
	// get redis conn
	c := r.pool.Get()
	defer c.Close()

	// check exists
	var (
		exists interface{}
		value  int64
		ok     bool
	)
	if exists, err = c.Do("exists", f.Filename); err != nil {
		log.Errorf("redis exists error, filename %s, %v", f.Filename, err)
		return err
	}
	if value, ok = exists.(int64); ok != true {
		log.Errorf("exists ret type is not int64")
		err = errors.ErrRedis
		return err
	}
	if value == 1 {
		log.Warningf("file exists filename %s", f.Filename)
		err = errors.ErrNeedleExist
		return err
	}

	// data to json
	b, err := json.Marshal(f)
	if err != nil {
		log.Errorf("meta file to json failed")
		return err
	}

	// put data
	if _, err = c.Do("set", f.Filename, b); err != nil {
		log.Errorf("set meta file failed %s %v", f.Filename, err)
		return err
	}
	return
}

func (r *RedisClient) DelDirSubitem(bucket string, dir string, subitem string) (num int, err error) {
	if strings.Index(dir, "/") < 0 || len(subitem) == 0 {
		log.Errorf("invalid parameter dir=%s, subitem=%s", dir, subitem)
		return -1, errors.ErrParam
	}

	item := make(map[string]string)
	item[dir] = subitem
	num, err = r.deleteHashKeys(item)
	if err != nil {
		log.Errorf("delete hash item failed, dir=%s, subitem=%s", dir, subitem)
		return num, err
	}
	return num, err
}

func (r *RedisClient) deleteHashKeys(keys map[string]string) (num int, err error) {
	c := r.pool.Get()
	defer c.Close()

	var ret interface{}
	var ivalue int64

	for key, value := range keys {
		ret, err = c.Do("hdel", key, value)
		ivalue, _ = ret.(int64)
		num += int(ivalue)
	}
	return num, err
}

func (r *RedisClient) existsHashDir(key string) (exists bool, err error) {

	var (
		ex       interface{}
		value    int64
		ok       bool
		pos      int
		hashname string
		hashkey  string
	)
	if strings.HasSuffix(key, "/") == false || strings.HasPrefix(key, "/") == false {
		return false, errors.ErrParam
	}

	pos = strings.LastIndex(key[:len(key)-1], "/")
	hashname = key[:pos+1]
	hashkey = key[pos+1:]

	c := r.pool.Get()
	defer c.Close()
	if ex, err = c.Do("hexists", hashname, hashkey); err != nil {
		return false, err
	}
	if value, ok = ex.(int64); ok != true {
		log.Errorf("exists ret type is not int64")
		err = errors.ErrRedis
		return false, err
	}
	if value == 1 {
		return true, nil
	}
	return false, nil
}

func (r *RedisClient) putDir(bucket string, filename string, filesize int64) (insert_times int, err error) {
	var (
		pos        int
		dir_exists bool
		keys       map[string]string
		hvalue     int64
	)
	if strings.HasSuffix(filename, "/") == true {
		return 0, errors.ErrParam
	}

	c := r.pool.Get()
	defer c.Close()

	insert_times = 0
	keys = make(map[string]string)
	pos = strings.LastIndex(filename, "/")
	dir_exists = false
	for {
		dir := filename[0 : pos+1]
		file := filename[pos+1:]
		isDir := strings.HasSuffix(file, "/")
		if isDir {
			hvalue = 0
		} else {
			hvalue = filesize
		}

		dir_exists, err = r.existsHashDir(dir)
		if err != nil {
			r.deleteHashKeys(keys)
			return 0, err
		}
		// put dir->subdir or file to ssdb
		if _, err = c.Do("hset", dir, file, hvalue); err != nil {
			log.Errorf("set dir info failed %s->%s %v", dir, file, err)
			r.deleteHashKeys(keys)
			return 0, err
		}
		insert_times += 1
		keys[dir] = file
		//fmt.Printf("dir=%s, file=%s, value=%d\n", dir, file, hvalue)

		if dir == "/" {
			break
		}
		pos = strings.LastIndex(dir[:len(dir)-1], "/")
		filename = dir
		if dir_exists {
			break
		}
	}
	return insert_times, nil
}

func (r *RedisClient) putNeedle(n *meta.Needle) (err error) {
	var (
		ret_exists interface{}
		exists     int64
		ok         bool
	)
	// check exists
	c := r.pool.Get()
	defer c.Close()
	if ret_exists, err = c.Do("exists", n.Key); err != nil {
		log.Errorf("redis exists error, key %d err %v", n.Key, err)
		return err
	}
	if exists, ok = ret_exists.(int64); ok != true {
		log.Errorf("type assertion error")
		err = errors.ErrRedis
		return err
	}
	if exists == 1 {
		err = errors.ErrNeedleExist
		return err
	}

	// data to json
	b, err := json.Marshal(n)
	if err != nil {
		log.Errorf("needle to json failed")
		return err
	}

	if _, err = c.Do("set", n.Key, b); err != nil {
		log.Errorf("needle to redis failed, %d %v", n.Key, err)
		return err
	}
	return
}

func (r *RedisClient) Get(bucket, filename string) (n *meta.Needle, f *meta.File, err error) {
	if f, err = r.getFile(bucket, filename); err != nil {
		return
	}
	if len(f.Chunks) == 0 {
		if n, err = r.getNeedle(f.Key); err == errors.ErrNeedleNotExist {
			log.Warningf("table not match: bucket: %s filename: %s", bucket, filename)
			r.delFile(bucket, filename)
		}
	}

	return
}

func (r *RedisClient) GetDirInfo(bucket, dir string) (ret meta.DirInfo, err error) {
	c := r.pool.Get()
	defer c.Close()

	res, err := c.Do("hkeys", dir)
	if err != nil {
		log.Errorf("redis hkeys error %v", err)
		err = errors.ErrRedis
		return ret, err
	}
	interface_list, ok := res.([]interface{})
	if !ok {
		log.Errorf("res type is not interface list %v, err %v", res, err)
		err = errors.ErrRedis
		return ret, err
	}

	if len(interface_list) == 0 {
		log.Warningf("dir %s is not exist", dir)
		return ret, errors.ErrDirNotExist
	}

	ret.Dir = dir
	ret.Files = make([]string, 0)
	ret.SubDirs = make([]string, 0)
	for _, interface_item := range interface_list {
		key, ok := interface_item.([]byte)
		if !ok {
			log.Errorf("ret type is not []byte %v, err %v", interface_item, err)
			err = errors.ErrRedis
			return ret, err
		}
		if strings.HasSuffix(string(key), "/") == true {
			ret.SubDirs = append(ret.SubDirs, string(key))
		} else {
			ret.Files = append(ret.Files, string(key))
		}
	}
	return ret, nil
}

func (r *RedisClient) Del(bucket, filename string) (err error) {
	var (
		f *meta.File
	)
	if f, err = r.getFile(bucket, filename); err != nil {
		return err
	}

	if err := r.delFile(bucket, filename); err != nil {
		log.Warningf("del file err %s %v", filename, err)
		return err
	}
	if len(f.Chunks) == 0 {
		err = r.delNeedle(f.Key)
	}
	return err
}

func (r *RedisClient) delNeedle(key int64) (err error) {
	c := r.pool.Get()
	defer c.Close()
	// TODO check return
	if _, err = c.Do("del", key); err != nil {
		log.Errorf("redis del error, key %d %v", key, err)
		return err
	}
	return
}

func (r *RedisClient) delFile(bucket, filename string) (err error) {
	// TODO select bucket

	c := r.pool.Get()
	defer c.Close()
	// del data
	if _, err = c.Do("del", filename); err != nil {
		log.Errorf("redis del error key %s %v", filename, err)
		return err
	}

	if strings.HasPrefix(filename, "/") == false {
		filename = "/" + filename
	}
	pos := strings.LastIndex(filename, "/")
	dir := filename[:pos+1]
	file := filename[pos+1:]
	if _, err = c.Do("hdel", dir, file); err != nil {
		log.Errorf("redis del error key %s %s %v", dir, file, err)
		return err
	}

	return nil
}

func (r *RedisClient) getNeedle(key int64) (n *meta.Needle, err error) {
	var (
		data       interface{}
		data_bytes []byte
		ok         bool
	)
	c := r.pool.Get()
	defer c.Close()
	// get data
	if data, err = c.Do("get", key); err != nil {
		log.Errorf("redis get error key %v, %v", key, err)
		return nil, err
	}
	// not exist
	if data_bytes, ok = data.([]byte); ok != true {
		log.Errorf("key %v is not exist.", key)
		err = errors.ErrNeedleNotExist
		return
	}
	// byte to json
	n = new(meta.Needle)
	if err = json.Unmarshal(data_bytes, n); err != nil {
		log.Errorf("json.Unmarshal failed")
	}
	return

}

func (r *RedisClient) getFile(bucket, filename string) (f *meta.File, err error) {
	var (
		data       interface{}
		data_bytes []byte
		ok         bool
	)
	c := r.pool.Get()
	defer c.Close()
	if data, err = c.Do("get", filename); err != nil {
		log.Errorf("redis get error filename %v, %v", filename, err)
		return nil, err
	}
	// not exist
	if data_bytes, ok = data.([]byte); ok != true {
		err = errors.ErrNeedleNotExist
		return
	}
	// byte to json
	f = new(meta.File)
	if err = json.Unmarshal(data_bytes, f); err != nil {
		log.Errorf("json.Unmarshal failed")
	}
	return
}
