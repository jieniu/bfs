# proxy接口


## 1 上传

### 1.1 请求

[POST] http://ip:port/file?path=/prefix/bucket/filename

| 参数         | 说明     |
| :--          | :--      |
| prefix       | xfs      |
| bucket       | 业务id   |
| filename     | 文件名   |
| Content-Type | 文件类型 |
| Body         | 文件数据 |


- 若文件名为空，则以"sha1sum.文件类型"作为文件名；
- 若文件名为以"/"结尾的目录名，则以"目录名/sha1sum.文件类型"作为文件名。

### 1.2 响应

| 参数     | 说明        |
| :--      | :--         |
| Code     | 200         |
| Location | 文件URI     |
| ETag     | 文件sha1sum |

### 1.3 内部实现
#### 参数检测
* Content-Type没有设置，返回400
* 读数据失败，返回400
* 数据内容大于MaxFileSize，返回413

文件名为空，则修改文件名为sha1sum.ext

- 由directory分配存储该文件的vid和对应的所有store；
- 向以上store集逐个上传该文件，直至全部成功。（串行上传，如果失败，目前没有删除元数据）
- 单文件时，需要在ssdb中保存filename->key的对应关系，带路径的文件需要添加dir->subdir；dir->filename的映射，例如一个路径/dir1/dir2/dir3/dir4/file，需要建立以下映射
```
/dir1/dir2/dir3/dir4/file   key
key                         needle
/dir1/dir2/dir3/dir4/       file
/dir1/dir2/dir3/            dir4/
/dir1/dir2/                 dir3/
/dir1/                      dir2/
```
### 1.4 错误
主要有三大类
* 参数错误
* 服务器间通信错误
    - 封包组包
    - 服务器间请求响应超时
* 内部错误（在libs/errors/errors.go文件中查看错误码）


## 2 删除

### 2.1 请求
#### 删除文件
[DELETE] http://ip:port/file?path=prefix/bucket/filename
#### 删除目录
[DELETE] http://ip:port/file?path=prefix/bucket/dir/

| 参数         | 说明   |
| :--          | :--    |
| prefix       | xfs    |
| bucket       | 业务id |
| filename     | 文件名 |
| dir/         | 目录名 |

### 2.2 响应

| 参数     | 说明        |
| :--      | :--         |
| Code     | 200         |

### 3.3 内部实现

- 向directory获取存储该文件的vid和对应的所有store；
- 向以上store集逐个请求删除该文件，直至全部成功。
- 如需删除目录，则递归删除目录及子目录及其下的文件，先删除子目录及文件，再删除父目录及文件


## 3 下载

### 3.1 请求

[GET] http://ip:port/file?path=prefix/bucket/filename

| 参数         | 说明           |
| :--          | :--            |
| prefix       | xfs            |
| bucket       | 业务id         |
| filename     | 文件名         |
| Range        | 请求的字节范围 |

### 3.2 响应

| 参数           | 说明                |
| :--            | :--                 |
| Code           | 200                 |
| Content-Type   | 文件类型            |
| Server         | xfs                 |
| Etag           | 所请求数据的sha1sum |
| Body           | 所请求的数据        |

### 3.3 内部实现

- 向directory获取存储该文件的vid和对应的所有可读store；
- 在以上store集中随机选择一个向其发起下载请求（本步骤需要将文件的range转化到块的range），若失败则逐个尝试直至成功。


## 4 获取文件尺寸/目录列表

### 4.1 请求

#### 文件尺寸
[HEAD] http://ip:port/fileinfo?path=prefix/bucket/filename

#### 目录列表
[HEAD] http://ip:port/fileinfo?path=prefix/bucket/dir/

| 参数         | 说明   |
| :--          | :--    |
| prefix       | xfs    |
| bucket       | 业务id |
| filename     | 文件名 |
| dir/         | 目录名 |

### 4.2 响应

| 参数           | 说明                           |
| :--            | :--                            |
| Code           | 200                            |
| Content-Type   | application/json;charset=utf-8 |
| Body           | {"filesize":文件字节数}        |

如果是目录，则返回以下json串
```
{
    "dir":"/dir/",
    "files": [
        "file1","file2","file3"
    ]
    "subdirs": [
        "subdir1","subdir2"
    ]
}
```


### 4.3 内部实现

#### 文件
向ssdb查询文件元信息，并返回文件大小。

#### 目录
为了可以快速获取目录及其子目录和文件，需要将key进行排序并存储，这样可以支持range操作，迅速取出某个目录下的所有信息，而ssdb的存储介质leveldb正好具备这样的特性

##### 复杂度
上面说了，因为数据是排序后存储在磁盘中，查询目录是一个range操作，所以查询第一个目录节点所需的复杂度则为整个查询操作的复杂度，LevelDB是一个logstructured-merge (LSM) ，复杂度和B-Tree一样，是O(logN)
