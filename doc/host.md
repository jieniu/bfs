## 部署系统环境


### 使用普通用户

新建普通用户：
```
# useradd xfs
# passwd xfs
```

将此用户加入sudo用户：
```
# visudo
```
在打开的文件中添加如下内容后保存退出：
```
xfs     ALL=(ALL)       ALL
```


### 准备相关文件

将相关文件传输到服务器：
```
$ scp -r xfs_env xfs@serverip:/home/xfs/
```

在服务器上解压文件：
```
$ cd xfs_env
$ tar -xf jdk-8u91-linux-x64.tar.xz
$ tar -xf zookeeper-3.4.8.tar.xz
$ tar -xvf ssdb-1.9.2.tar.xz
```


### 清除防火墙规则

```
$ sudo iptables -F
```


### 设置Java环境：

zookeeper依赖于Java。

向`~/.bashrc`添加以下内容：
```
# java
export JAVA_HOME=$HOME/xfs_env/jdk1.8.0_91
export JRE_HOME=$JAVA_HOME/jre
export CLASSPATH=.:$JAVA_HOME/lib/dt.jar:$JAVA_HOME/lib/tools.jar:$JRE_HOME/lib:$CLASSPATH
export PATH=$JAVA_HOME/bin:$PATH
```


### 运行zookeeper：

使用示例配置文件：
```
$ cd ~/xfs_env/zookeeper-3.4.8
$ cp conf/zoo_sample.cfg conf/zoo.cfg
```

修改和添加`conf/zoo.cfg`的以下配置：
```
dataDir=/home/xfs/xfs_data/zookeeper
maxClientCnxns=1024
server.1=10.10.30.112:2901:3901
server.2=10.10.30.113:2901:3901
server.3=10.10.30.114:2901:3901
```

创建zookeeper数据目录并写入本IP在zookeeper集群中的序号到特定文件：
```
$ mkdir -p ~/xfs_data/zookeeper && echo "1" > ~/xfs_data/zookeeper/myid
```

启动zookeeper并查看状态：
```
$ ./bin/zkServer.sh start ./conf/zoo.cfg
$ ./bin/zkServer.sh status
```

通过zookeeper命令行创建`/pitchfork`节点：
```
$ ./bin/zkCli.sh -server 10.10.30.112:2181
[zk ...] create /pitchfork ""
```


### 运行ssdb

编译ssdb：
```
$ cd ~/xfs_env/ssdb-1.9.2
$ make
```

在`ssdb.conf`中`server`小节中按如下配置：
```
ip: 0.0.0.0
```

将ssdb作为守护进程运行：
```
$ ./ssdb-server -d ssdb.conf
```



## 部署XFS环境


### 准备相关文件

将相关文件传输到服务器：
```
scp -r xfs_bin xfs@serverip:/home/xfs/
```

在每台机器上创建log目录：
```
$ mkdir -p ~/xfs_data/log
```


### 运行store

在`store.toml`中修改如下配置，其中`ServerId`须每台机器不同，`myip`须填写自身IP：
```
PprofListen  = "myip:6060"
StatListen   = "myip:6061"
ApiListen    = "myip:6062"
AdminListen  = "myip:6063"

[Store]
VolumeIndex      = "/home/xfs/xfs_data/store/volume.idx"
FreeVolumeIndex  = "/home/xfs/xfs_data/store/free_volume.idx"

[Zookeeper]
ServerId  = "47E273ED-CD3A-4D6A-94CE-554BA9B19114"
Addrs = [
    "zk1:2181",
    "zk2:2181",
    "zk3:2181"
]
```

创建存储store数据的目录并运行store：
```
$ mkdir -p ~/xfs_data/store
$ nohup ./store -c=store.toml -log_dir=~/xfs_data/log/ > ~/xfs_data/log/nohup_store.log 2>&1 &
```


### 初始化store

在各机器上准备磁盘空间：
```
$ sudo mkdir /data1/xfs1
$ sudo chown xfs:xfs /data1/xfs1
```

初始化store使用`~/xfs_bin/ops`模块，并不需要此模块运行于集群中。在`commons/config.py`中修改如下配置：
```
zk_hosts = 'zk1:2181'
```

运行`ops`模块的`server`：
```
$ cd ~/xfs_bin/ops
$ python runserver.py
```

在`test/ops_initialization.py`中按如下设置：
```
def space():
    value = {"ips":"ip1,ip2","dirs":"/data1/xfs1/","size":"75G"}

def groups():
    value = {"ips":"ip1,ip2","copys":2,"rack":1}
```

运行`ops`模块的`client`并依次调用以下方法，其中`volumes`方法可多次调用：
```
$ cd test
$ python
>>> import ops_initialization
>>> ops_initialization.space()
'{\n  "errorMsg": "", \n  "status": "ok"\n}\n'
>>> ops_initialization.groups()
'{"status": "ok", "errorMsg": "", "content": [{"ips": "10.10.30.115,10.10.30.114", "groupid": 1}]}'
>>> ops_initialization.volumes()
'{"status": "ok", "errorMsg": ""}'
```


### 运行gosnowflake

创建存储gosnowflake数据的目录：
```
$ mkdir -p ~/xfs_data/gosnowflake
```

在`gosnowflake.conf`中修改如下配置：
```
[base]
dir /home/xfs/xfs_data/gosnowflake/
log /home/xfs/xfs_data/log/gosnowflake.xml

[zookeeper]
addr 10.10.30.112:2181
```

运行gosnowflake：
```
$ nohup ./gosnowflake -conf=./gosnowflake.conf > ~/xfs_data/log/nohup_gosnowflake.log 2>&1 &
```


### 运行directory

在`directory.toml`中修改如下配置：
```
ApiListen = "myip:6065
PprofListen = "myip:6066"

[snowflake]
ZkAddrs = [
    "zk1:2181",
    "zk2:2181",
    "zk3:2181"
]

[Zookeeper]
Addrs = [
    "zk1:2181",
    "zk2:2181",
    "zk3:2181"
]

[redis]
Addr = "ssdbip:8888"
```

运行directory：
```
$ nohup ./directory -c=directory.toml -log_dir=~/xfs_data/log/ > ~/xfs_data/log/nohup_directory.log 2>&1 &
```


### 运行pitchfork

在`pitchfork.toml`中修改如下配置：
```
[Zookeeper]
Addrs = [
    "zk1:2181",
    "zk2:2181",
    "zk3:2181"
]
```

运行pitchfork：
```
$ nohup ./pitchfork -c=pitchfork.toml -log_dir=~/xfs_data/log/ > ~/xfs_data/log/nohup_pitchfork.log 2>&1 &
```


### 运行proxy

在`proxy.toml`中修改如下配置：
```
PprofListen = "myip:2231"
HttpAddr = "myip:2232"
XfsAddr = "directoryip:6065"
Domain = "http://myip:2232/"

[redis]
Addr = "ssdbip:8888"
```

运行proxy：
```
$ nohup ./proxy -c=proxy.toml -log_dir=~/xfs_data/log/ > ~/xfs_data/log/nohup_proxy.log 2>&1 &
```
