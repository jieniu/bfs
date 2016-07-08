package redis_c

import (
	"bfs/directory/conf"
	"bfs/libs/errors"
	"bfs/libs/meta"
	"encoding/json"
	redis "github.com/garyburd/redigo/redis"
	log "github.com/golang/glog"
	"time"
)

type RedisClient struct {
}

var (
	pool   *redis.Pool
	config *conf.Config
)

func Init(conf *conf.Config) error {
	config = conf
	log.Infof("redis config: maxidle[%s], timeout[%s], addr[%s]", config.Redis.MaxIdle, config.Redis.Timeout, config.Redis.Addr)
	pool = &redis.Pool{
		MaxIdle:     config.Redis.MaxIdle,
		IdleTimeout: time.Duration(config.Redis.Timeout) * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", config.Redis.Addr)
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
	return nil
}

func Close() {
	pool.Close()
}

func NewRedisClient() *RedisClient {
	return &RedisClient{}
}

func (r *RedisClient) Put(bucket string, f *meta.File, n *meta.Needle) (err error) {
	if err = r.putFile(bucket, f); err != nil {
		log.Warningf("redis putFile error, bucket: %s, filename: %s", bucket, f.Filename)
		return
	}
	if err = r.putNeedle(n); err != errors.ErrNeedleExist && err != nil {
		log.Warningf("insert table not match: bucket: %s  filename: %s", bucket, f.Filename)
		r.delFile(bucket, f.Filename)
	}
	return
}

func (r *RedisClient) putFile(bucket string, f *meta.File) (err error) {
	// get redis conn
	c := pool.Get()
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

func (r RedisClient) putNeedle(n *meta.Needle) (err error) {
	var (
		ret_exists interface{}
		exists     int64
		ok         bool
	)
	// check exists
	c := pool.Get()
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
	if n, err = r.getNeedle(f.Key); err == errors.ErrNeedleNotExist {
		log.Warningf("table not match: bucket: %s filename: %s", bucket, filename)
		r.delFile(bucket, filename)
	}

	return
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
	err = r.delNeedle(f.Key)
	return err
}

func (r *RedisClient) delNeedle(key int64) (err error) {
	c := pool.Get()
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

	c := pool.Get()
	defer c.Close()
	// del data
	if _, err = c.Do("del", filename); err != nil {
		log.Errorf("redis del error key %s %v", filename, err)
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
	c := pool.Get()
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
	c := pool.Get()
	defer c.Close()
	// TODO select db
	// get data
	if data, err = c.Do("get", filename); err != nil {
		log.Errorf("redis get error filename %v, %v", filename, err)
		return nil, err
	}
	// not exist
	if data_bytes, ok = data.([]byte); ok != true {
		log.Errorf("key %v is not exist.", filename)
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
