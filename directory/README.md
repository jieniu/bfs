# directory接口

[toc]

---


## 1 上传

### 1.1 请求

[POST] http://ip:port/upload

| 参数         | 说明                |
| :--          | :--                 |
| Content-Type | multipart/form-data |
| bucket       | 业务id              |
| filename     | 文件名              |
| sha1         | 文件sha1sum         |
| mine         | 文件类型            |
| filesize     | 文件字节数          |

### 1.2 响应

| 参数         | 说明                           |
| :--          | :--                            |
| Content-Type | application/json;charset=utf-8 |
| Body         | {"ret":1,"key":%d,"cookie":%d,"vid":%d,"stores":["%s","%s",...]} |

### 1.3 内部实现

- 调度group，分配vid，获取所有对应的store；
- 分配文件key和cookie；
- 将文件元信息、needle信息插入ssdb。
- 将分配的store返回给proxy



## 2 删除

### 2.1 请求

[POST] http://ip:port/del

| 参数         | 说明                |
| :--          | :--                 |
| Content-Type | multipart/form-data |
| bucket       | 业务id              |
| filename     | 文件名              |

### 2.2 响应

| 参数         | 说明                           |
| :--          | :--                            |
| Content-Type | application/json;charset=utf-8 |
| Body         | {"ret":1,"key":%d,"cookie":%d,"vid":%d,"stores":["%s","%s",...]} |

### 2.3 内部实现
- 根据末尾字符判断文件还是目录
- 目录则先获取目录下文件和子目录，然后递归子目录下的子目录，删除所有子目录下的文件
- 如果是文件直接删除文件
- 删除信息包含ssdb中的文件元数据和目录与文件/目录与子目录的信息
- 如果有大文件，则分别删除对应的大文件的块，及大文件的元数据



## 3 下载

### 3.1 请求

[GET] http://ip:port/get

| 参数         | 说明                |
| :--          | :--                 |
| Content-Type | multipart/form-data |
| bucket       | 业务id              |
| filename     | 文件名              |

### 3.2 响应

| 参数         | 说明                           |
| :--          | :--                            |
| Content-Type | application/json;charset=utf-8 |
| Body         | {"ret":1,"key":%d,"cookie":%d,"vid":%d,"stores":["%s","%s",...],"mine":"%s","sha1":"%s"} |

### 3.3 内部实现

- 从ssdb获取文件元信息和needle元信息；
- 通过vid获取所有对应的可读store。
- 如果range跨块了，则返回ErrFileTooLarge错误
- 如果是大文件，则要定位到该range定位到的块


## 4 Ping

### 4.1 请求

[GET] http://ip:port/ping

### 4.2 响应

| 参数         | 说明                           |
| :--          | :--                            |
| Content-Type | application/json;charset=utf-8 |
| Body         | {"code":0}                     |



## 5 从zookeeper同步信息

### 5.1 同步时机

directory会从zookeeper中同步信息，同步的时机为：
- 定时；
- zookeeper信息发生变化时。

### 5.2 同步内容

directory会从zookeeper中同步以下顶层节点：
- /rack
- /volume
- /group

### 5.3 打分

- 根据group下store写次数，写延迟，剩余空间计算group权重，产生可写group列表；
- 以后写请求的时候会根据以上列表进行分配。

### 5.4 打分及分配算法

1. group的剩余空间对32GB求余的结果为剩余空间分值，超过1000分则设为1000分；
2. group的单个写入请求的平均延迟毫秒数作为延迟分值，超过1000分则设为1000分，低于100分则设为0分；
3. 以上剩余空间分值减去延迟分值，得到总分；
4. 该group的权重为以上总分，若总分小于0则权重为0。
