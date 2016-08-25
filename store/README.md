# store接口

[toc]

---


## 1 数据操作接口

### 1.1 上传

请求：
[POST] http://ip:port/upload

| 参数         | 说明                |
| :--          | :--                 |
| Content-Type | multipart/form-data |
| vid          | volume id           |
| key          | 文件key             |
| cookie       | 文件cookie          |
| file         | 文件数据            |

响应：

| 参数         | 说明                           |
| :--          | :--                            |
| Code         | 200                            |
| Content-Type | application/json;charset=utf-8 |
| Body         | {"ret":1}                      |

内部实现：
1. 检查文件大小是否超出最大文件大小,以及其他参数是否合法  
2. 根据http请求中所负载的文件元信息及文件数据，构建needle并更新superblock及superblock的索引文件（数据写入是同步的，更新索引文件是异步的），然后更新volume状态。
3. 更新TotalWriteProcessed；TotalWriteBytes；TotalWriteDelay统计信息


### 1.2 批量上传

请求：
[POST] http://ip:port/uploads

| 参数         | 说明                |
| :--          | :--                 |
| Content-Type | multipart/form-data |
| vid          | volume id           |
| keys         | 一组文件的key       |
| cookies      | 一组文件的cookie    |
| file         | 一组文件的数据      |

响应：

| 参数         | 说明                           |
| :--          | :--                            |
| Code         | 200                            |
| Content-Type | application/json;charset=utf-8 |
| Body         | {"ret":1}                      |

内部实现：
根据http请求中所负载的文件元信息及文件数据，构建一组needle并更新superblock及superblock的索引文件，然后更新volume状态。


### 1.3 删除

请求：

[POST] http://ip:port/del
| 参数         | 说明                              |
| :--          | :--                               |
| Content-Type | application/x-www-form-urlencoded |
| vid          | volume id                         |
| key          | 文件的key                         |

响应：

| 参数         | 说明                           |
| :--          | :--                            |
| Code         | 200                            |
| Content-Type | application/json;charset=utf-8 |
| Body         | {"ret":1}                      |

内部实现：
根据http请求中所负载的文件元信息，将内存中哈希表中对应needle标记为删除，并将superblock中的needle标记为删除，然后更新volume状态。superblock的索引文件暂不处理，由今后读取needle时再做更新。


### 1.4 下载

[GET] http://ip:port/get

| 参数         | 说明           |
| :--          | :--            |
| vid          | volume id      |
| key          | 文件的key      |
| cookie       | 文件的cookie   |
| Range        | 请求的字节范围 |

响应：

| 参数           | 说明         |
| :--            | :--          |
| Code           | 200          |
| Content-Length | Body的字节数 |
| Body           | 所请求的数据 |

内部实现：
1. 检查参数是否合法  
2. 判断needle或者volume是否存在，对比cookie是否一致
3. 根据http请求中所负载的文件元信息，从volume中读取needle，并按请求的字节范围返回文件数据和字节数。



## 2 Admin接口

### 2.1 添加自由卷

请求：
[POST] http://ip:port/add_free_volume

| 参数         | 说明                              |
| :--          | :--                               |
| Content-Type | application/x-www-form-urlencoded |
| n            | volume数量                        |
| bdir         | 存储superblock的目录              |
| idir         | 存储superblock的索引文件的目录    |

响应：

| 参数         | 说明                           |
| :--          | :--                            |
| Code         | 200                            |
| Content-Type | application/json;charset=utf-8 |
| Body         | {"ret":1}                      |

内部实现：
在指定的目录下创建指定数量的free volume文件（包含superblock和superblock的索引文件），然后更新记录free volume信息的索引文件。


### 2.2 添加卷

请求：
[POST] http://ip:port/add_volume

| 参数         | 说明                              |
| :--          | :--                               |
| Content-Type | application/x-www-form-urlencoded |
| vid          | volume id                         |

响应：

| 参数         | 说明                           |
| :--          | :--                            |
| Code         | 200                            |
| Content-Type | application/json;charset=utf-8 |
| Body         | {"ret":1}                      |

内部实现：
找到可用的free volume文件，重命名并更新记录free volume和volume信息的索引文件。


### 2.3 压缩卷

请求：
[POST] http://ip:port/compact_volume

| 参数         | 说明                              |
| :--          | :--                               |
| Content-Type | application/x-www-form-urlencoded |
| vid          | volume id                         |

响应：

| 参数         | 说明                           |
| :--          | :--                            |
| Code         | 200                            |
| Content-Type | application/json;charset=utf-8 |
| Body         | {"ret":1}                      |

内部实现：
找到可用的free volume作为新volume，将旧volume的未被标记为删除的needle复制到新volume，完成后更新zookeeper及记录free volume和volume信息的索引文件。然后等待一定时间（10s）后删除旧volume，并添加free volume，保持volume总数和free volume数量不变。


### 2.4 BulkVolume

请求：
[POST] http://ip:port/bulk_volume

| 参数         | 说明                              |
| :--          | :--                               |
| Content-Type | application/x-www-form-urlencoded |
| vid          | volume id                         |
| bfile        | superblock文件                    |
| ifile        | superblock的索引文件              |

响应：

| 参数         | 说明                           |
| :--          | :--                            |
| Code         | 200                            |
| Content-Type | application/json;charset=utf-8 |
| Body         | {"ret":1}                      |

内部实现：
将volume与volume id相关联，然后更新记录volume信息的索引文件及zookeeper。


## 3 Stat接口

### 3.1 查询信息

请求：
[GET] http://ip:port/info

响应：

| 参数         | 说明                           |
| :--          | :--                            |
| Code         | 200                            |
| Content-Type | application/json;charset=utf-8 |
| server       | store的信息                    |
| volumes      | 所有volume的信息               |
| free_volumes | 所有free volume的信息          |

内部实现：
直接返回整个store、store上的所有volume、store上的所有free volume的信息。


## 4 理论基础

### 4.1 SuperBlock

store模块是存储小文件的核心模块，旨在减少文件数，减少磁盘IO次数。为了达到这个目的，引入了超大块（super block）的概念，顺序写还可以提升磁盘写效率。

![659877-a89b8fd35f4daf74.png.jpeg-26.9kB][1]

| Filed    | explanation                             |
| :--      | :--                                     |
| magic    | header magic number used for checksum   |
| cookie   | random number to mitigate brute force lookups |
| key      | 64bit photo id                          |
| flag     |   signifies deleted status              |
| size     | data size                               |
| data     | the actual photo data                   |
| magic    | footer magic number used for checksum   |
| checksum | used to check integrity                 |
| padding  | total needle size is aligned to 8 bytes |

为了节省存储空间，我们希望单个SuperBlock的偏移量控制在4个字节（即uint32）范围内，而4个字节只能寻址4GB存储空间，如果想要寻址更多的空间，可以按照8个或更多字节对齐，我们初步按照8字节对齐，这样一个SuperBlock有32GB存储空间。


### 4.2 SuperBlock的索引文件

有了SuperBlock概念，我们将其命名为volume，读取文件时，只需要知道该文件所在的volume及相应的偏移量即可，所以Node中需要存储`<key, volume_id+offset>`的hash映射，这里有个优化点，因为文件数得到了控制，我们可以预先打开所有volume文件，这样读取一个文件只需一次fseek操作即可。

hash映射的重新加载：如果store服务器重启，会面临重新加载hash映射的情况，如果逐个扫描32GB volume文件来建立映射表会造成启动缓慢的问题，这里可以创建一个索引表文件，仅存储映射的元信息即可。

![659877-223b198831a680a3.png.jpeg-17.1kB][2]

为了提升写入效率，索引文件的写入是异步的，这样可能会存在索引文件和Volume中的索引不一致的问题，建立hash映射时，可以通过：
1. 读取索引文件；
2. 读取Volume中的最后几个文件，补充不一致的信息。


### 4.3 文件的读写删

读文件：
根据hash映射表可以找到文件并进行读取。

写文件：
追加到volume并添加hash映射。

删文件：
将hash表中对应needle标记为删除，并将SuperBlock中对应needle标记为删除；SuperBlock的索引文件暂不处理，由今后读取needle时再做更新。


## 5 启动流程

### 5.1 双向同步

启动自main函数开始，最关键的一步在对`NewStore`函数的调用中：先从卷索引文件中的信息，将卷信息添加到zookeeper；再将zookeeper中的卷信息更新回卷索引文件。即通过双向同步消除启动时卷索引文件和zookeeper中的卷信息不同步的情况。

![newst.png-205.1kB][3]


### 5.2 创建或打开文件

在双向同步的过程中，会创建或打开superblock和superblock的索引文件，如果索引文件的内容落后于superblock，则从superblock中取得索引文件中所缺失的内容并补全。

![newvo.png-164kB][4]



### 5.3 启动http服务

在完成以上工作之后，就依次启动用于状态获取、数据操作、管理的http服务，监听在不同端口上。并更新zookeeper中的信息。


  [1]: http://static.zybuluo.com/yvan/mhf5yz75z105hiasqjnavvxn/659877-a89b8fd35f4daf74.png.jpeg
  [2]: http://static.zybuluo.com/yvan/uc5c162ydnlo0h4ph0c2pyyk/659877-223b198831a680a3.png.jpeg
  [3]: http://static.zybuluo.com/yvan/7dkhngisbemc1azcvu2b7j2h/newst.png
  [4]: http://static.zybuluo.com/yvan/7f1wv44ck49tr78l1387zxkq/newvo.png
