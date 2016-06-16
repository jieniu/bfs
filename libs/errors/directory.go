package errors

const (
	// hbase
	RetHBase = 30100
	// id
	RetIdNotAvailable = 30200
	// store
	RetStoreNotAvailable = 30300
	// zookeeper
	RetZookeeperDataError = 30400
	// redis
	RetRedis = 30500
)

var (
	// hbase
	ErrHBase = Error(RetHBase)
	// redis
	ErrRedis = Error(RetRedis)
	// id
	ErrIdNotAvailable = Error(RetIdNotAvailable)
	// store
	ErrStoreNotAvailable = Error(RetStoreNotAvailable)
	// zookeeper
	ErrZookeeperDataError = Error(RetZookeeperDataError)
)
