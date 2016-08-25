#ifndef __XFSC_H__
#define __XFSC_H__


#ifdef __cplusplus
extern "C" {
#endif


#include <stdint.h>


/**
 * \brief  错误码.
 */
enum XFSC_ERROR {
    XFSC_ERROR_OK               =  -1,  /* No error */
    XFSC_ERROR_NO_MEMORY        =  -2,  /* No enough memory */
    XFSC_ERROR_PARAM_INVALID    = -11,  /* Parameters invalid */
    XFSC_ERROR_PROTO_INVALID    = -12,  /* Protocol invalid */
    XFSC_ERROR_PREFIX_INVALID   = -13,  /* Prefix invalid */
    XFSC_ERROR_FILENAME_INVALID = -14,  /* Filename invalid */
    XFSC_ERROR_UP_FILE_FAIL     = -21,  /* Upload file fail */
    XFSC_ERROR_FILE_STAT_FAIL   = -22,  /* Local file stat fail */
    XFSC_ERROR_FILE_OPEN_FAIL   = -23,  /* Local file open fail */
    XFSC_ERROR_DEL_FILE_FAIL    = -31,  /* Delete file fail */
    XFSC_ERROR_INFO_FILE_FAIL   = -41,  /* Get file info fail */
    XFSC_ERROR_DOWN_FILE_FAIL   = -51,  /* Download file fail */
    XFSC_ERROR_FILE_NOT_FOUND   = -61,  /* File not found, 404 */
};


/**
 * \brief  配置项，用于实例初始化.
 */
typedef struct _xfsc_config_s {
    int32_t thread_num;       /* 使用的线程数               */
    int64_t max_needle_size;  /* 文件超过此字节数则分片上传 */
    int32_t open_debug;       /* 是否输出详细调试信息           */
} xfsc_config_t;


/**
 * \brief  回调函数组.
 */
typedef struct _xfsc_callbacks_s {

    /**
     * \brief       异步通用回调函数.
     *
     * \param arg   回传所在接口的调用者信息.
     * \param err   错误码.
     * \param json  输出json格式的信息.
     * \param out   存储输出数据的buffer，若无输出数据则为NULL.
     * \param size  输出数据的字节数.
     * \return      void.
     */
    void (*callback)(void *arg, int32_t err, const char *json, const char *out, int64_t size);

    /**
     * \brief              log回调函数.
     *
     * \param module       输出log的模块名称.
     * \param millisecond  当前时刻距开始运行时刻的差值，单位为毫秒.
     * \param msg          存储log字符串的buffer.
     * \return             void.
     */
    void (*callback_log)(const char *module, uint64_t millisecond, const char *msg);
} xfsc_callbacks_t;


/**
 * \brief  服务器及文件信息.
 */
typedef struct _xfsc_file_s {
    const char *proto;     /* "http"                        */
    const char *ip;        /* 服务器IP                      */
    int32_t     port;      /* 服务端口号                    */
    const char *prefix;    /* "xfs"                         */
    const char *buck;      /* 业务id，例如"live"            */
    const char *filename;  /* 文件名，可包含所在目录的路径，
                              或以'/'结尾表示目录           */
} xfsc_file_t;


/**
 * \brief  保存xfsc实例信息的结构体.
 */
typedef struct _xfsc_s xfsc_t;


/**
 * \brief      将错误码转换为错误信息.
 *
 * \param err  错误码.
 * \return     指向错误信息字符串的指针.
 */
const char *xfsc_perror(enum XFSC_ERROR err);


/**
 * \brief         初始化，完成后返回.
 *
 * \param xfsc    维护此实例信息的结构体.
 * \param config  一些参数.
 * \param cbs     用于回调的函数集.
 * \return        错误码.
 */
int32_t xfsc_init(xfsc_t **xfsc, xfsc_config_t config, xfsc_callbacks_t cbs);


/**
 * \brief       销毁实例，完成后返回.
 *
 * \param xfsc  保存此实例信息的结构体.
 * \return      错误码.
 */
int32_t xfsc_destroy(xfsc_t *xfsc);


/**
 * \brief           上传一个文件，立即返回；上传完成后触发回调函数.
 *
 * \param xfsc      保存此实例信息的结构体.
 * \param arg       保存调用者信息，会由回调函数返回.
 * \param file      服务器及文件信息.
 * \param path      要上传的本地文件路径.
 * \return          当前等待任务数或错误码.
 * \async callback  \param arg   即以上arg参数.
 *                  \param err   错误码.
 *                  \param json  { "location": "/filename" }.
 *                  \param out   NULL.
 *                  \param size  0.
 *                  \return      void.
 */
int32_t xfsc_upload_file(xfsc_t *xfsc, void *arg, const xfsc_file_t *file, const char *path);


/**
 * \brief           分次顺序上传一个文件，立即返回；上传完成后触发回调函数.
 *                  此接口未完成.
 *
 * \param xfsc      保存此实例信息的结构体.
 * \param arg       保存调用者信息，会由回调函数返回.
 * \param uuid      文件唯一标识符，若参数in中包含完整文件数据，则可设为NULL.
 * \param file      服务器及文件信息.
 * \param in        文件数据buffer.
 * \param size      文件数据字节数.
 * \param complete  是否已提供完整的文件，是则设为1，否则设为0.
 * \return          当前等待任务数或错误码.
 * \async callback  \param arg   即以上arg参数.
 *                  \param err   错误码.
 *                  \param json  { "location": "/filename" }.
 *                  \param out   NULL.
 *                  \param size  0.
 *                  \return      void.
 */
int32_t xfsc_upload_buffer(xfsc_t *xfsc, void *arg, const char *uuid, const xfsc_file_t *file, const char *in, int64_t size, int32_t complete);


/**
 * \brief           删除一个文件或目录，立即返回；删除完成后触发回调函数.
 *
 * \param xfsc      保存此实例信息的结构体.
 * \param arg       保存调用者信息，会由回调函数返回.
 * \param file      服务器及文件信息.
 * \return          当前等待任务数或错误码.
 * \async callback  \param arg   即以上arg参数.
 *                  \param err   错误码.
 *                  \param json  NULL.
 *                  \param out   NULL.
 *                  \param size  0.
 *                  \return      void.
 */
int32_t xfsc_delete(xfsc_t *xfsc, void *arg, const xfsc_file_t *file);


/**
 * \brief           获取文件大小或目录内容列表，立即返回；获取完成后触发回调函数.
 *
 * \param xfsc      保存此实例信息的结构体.
 * \param arg       保存调用者信息，会由回调函数返回.
 * \param file      服务器及文件信息.
 * \return          当前等待任务数或错误码.
 * \async callback  \param arg   即以上arg参数.
 *                  \param err   错误码.
 *                  \param json  {
 *                                    "location": "/filename",
 *                                    "size"    : 12345,  // 文件字节数
 *                               }  // 适用于文件
 *                               or
 *                               {
 *                                    "location": "/dir/",
 *                                    "file"    : [ "file1", "file2" ],
 *                                    "subdir"  : [ "dir1", "dir2" ]
 *                               }  // 适用于目录
 *                  \param out   NULL.
 *                  \param size  0.
 *                  \return      void.
 */
int32_t xfsc_info(xfsc_t *xfsc, void *arg, const xfsc_file_t *file);


/**
 * \brief           下载文件，立即返回；下载完成后触发回调函数.
 *
 * \param xfsc      维护此实例信息的结构体.
 * \param arg       保存调用者信息，会由回调函数返回.
 * \param file      服务器及文件信息.
 * \param offset    起始位置偏移量.
 * \param size      请求的字节数，设为0则表示请求至文件结束处.
 * \return          当前等待任务数或错误码.
 * \async callback  \param arg   即以上arg参数.
 *                  \param err   错误码.
 *                  \param json  NULL.
 *                  \param out   存储文件数据的buffer.
 *                  \param size  文件数据字节数.
 *                  \return      void.
 */
int32_t xfsc_download(xfsc_t *xfsc, void *arg, const xfsc_file_t *file, int64_t offset, int64_t size);


#ifdef __cplusplus
}
#endif  /* __cplusplus */


#endif  /* __XFSC_H__ */
