#define __STDC_LIMIT_MACROS
#define __STDC_FORMAT_MACROS

#include "xfsc.h"
#include "queue.h"
#include "http_parser.h"
#include "sha1.h"
#include "ylog/ylog.h"

#include <curl/curl.h>
#include <json/json.h>
#include <string>
#include <cstdio>
#include <cstring>
#include <cstdlib>
#include <unistd.h>
#include <pthread.h>
#include <fcntl.h>
#include <inttypes.h>
#include <sys/stat.h>


using namespace std;


struct _xfsc_s {
    xfsc_config_t config;
    xfsc_callbacks_t cbs;

    xqueue_t *xqueue;
    ylog_t *ylog;
    bool stop;
};


enum XFSC_REQ_TYPE {
    XFSC_REQ_TYPE_UPLOAD_INFO,
    XFSC_REQ_TYPE_UPLOAD_FILE,
    XFSC_REQ_TYPE_UPLOAD_BUFFER,
    XFSC_REQ_TYPE_DELETE,
    XFSC_REQ_TYPE_HEAD,
    XFSC_REQ_TYPE_DOWNLOAD,
};


typedef struct {
    void *arg;
    string uri;
    string uri_info;
    string name;
    string path;
} data_upload_file_t;


typedef struct {
    void *arg;
    string uri;
} data_delete_t;


typedef struct {
    void *arg;
    string uri;
} data_head_t;


typedef struct {
    void *arg;
    string uri;
    int64_t offset;
    int64_t size;
} data_download_t;


typedef struct {
    enum XFSC_REQ_TYPE type;
    void *data;
} data_common_t;


typedef struct {
    char *buf;
    size_t pos;
    size_t size;
} curl_user_data_t;



static void digest_to_hex(const char *in, size_t len, char *out)
{
    for (size_t i = 0; i < len * 2; i++) {
        int val = (i%2) ? in[i/2]&0x0f : (in[i/2]&0xf0)>>4;
        out[i] = (val<10) ? '0'+val : 'a'+(val-10);
    }
}



static size_t curl_write_data_cb(void *buf, size_t size, size_t nmemb, void *userp)
{
    curl_user_data_t *data = (curl_user_data_t *)userp;
    size_t len = size * nmemb;

    if (data->pos + len > data->size) {
        void *buf_new = realloc(data->buf, 2 * (data->size + len));
        if (!buf_new)
            return 0;
        data->buf = (char *)buf_new;
        data->size = 2 * (data->size + len);
    }

    memcpy(data->buf + data->pos, buf, len);
    data->pos += len;

    return len;
}



static void curl_reset_userp(curl_user_data_t *userp)
{
    userp->pos = 0;
}



static CURL *curl_start()
{
    CURL *curl = curl_easy_init();
    if (!curl)
        return 0;

    return curl;
}



static void curl_stop(CURL *curl)
{
    if (curl)
        curl_easy_cleanup(curl);
}



static int32_t curl_do(CURL *curl, curl_user_data_t *userp, const char *url, enum XFSC_REQ_TYPE type, const char *extra, size_t len, bool open_debug=true)
{
    // reset user buffer
    curl_reset_userp(userp);

    // reset all setting
    curl_easy_reset(curl);

    if (open_debug)
        curl_easy_setopt(curl, CURLOPT_VERBOSE, 1L);
    curl_easy_setopt(curl, CURLOPT_NOSIGNAL, 1L);
    curl_easy_setopt(curl, CURLOPT_TIMEOUT, 20L);  // 2 seconds
    curl_easy_setopt(curl, CURLOPT_HEADER, 1L);
    curl_easy_setopt(curl, CURLOPT_WRITEFUNCTION, curl_write_data_cb);
    curl_easy_setopt(curl, CURLOPT_WRITEDATA, userp);

    curl_easy_setopt(curl, CURLOPT_URL, url);

    curl_slist *headers = 0;
    switch(type) {
        case XFSC_REQ_TYPE_UPLOAD_BUFFER:
            headers = curl_slist_append(headers,
                    "Content-Type: application/octet-stream");
            headers = curl_slist_append(headers, "Expect:");
            curl_easy_setopt(curl, CURLOPT_HTTPHEADER, headers);
            curl_easy_setopt(curl, CURLOPT_POST, 1L);
            curl_easy_setopt(curl, CURLOPT_POSTFIELDS, extra);
            curl_easy_setopt(curl, CURLOPT_POSTFIELDSIZE, len);
            break;
        case XFSC_REQ_TYPE_DELETE:
            curl_easy_setopt(curl, CURLOPT_CUSTOMREQUEST, "DELETE");
            break;
        case XFSC_REQ_TYPE_HEAD:
            curl_easy_setopt(curl, CURLOPT_HTTPGET, 1L);
            curl_easy_setopt(curl, CURLOPT_HEADER, 0L);
            break;
        case XFSC_REQ_TYPE_DOWNLOAD:
            curl_easy_setopt(curl, CURLOPT_HTTPGET, 1L);
            curl_easy_setopt(curl, CURLOPT_HEADER, 0L);
            if (len > 0)
                curl_easy_setopt(curl, CURLOPT_RANGE, extra);
            break;
        default:
            break;
    }

    curl_easy_perform(curl);

    if (type == XFSC_REQ_TYPE_UPLOAD_BUFFER)
        curl_slist_free_all(headers);

    int code;
    curl_easy_getinfo(curl, CURLINFO_RESPONSE_CODE, &code);
    return code;
    // FIXME! need check every curl call
}



static string sha1sum(const char *in, size_t len)
{
    char digest[SHA1_SIZE];
    char sha1[SHA1_SIZE * 2];
    SHA1_CTX ctx;
    SHA1_Init(&ctx);
    SHA1_Update(&ctx, (uint8_t *)in, len);
    SHA1_Final((uint8_t *)digest, &ctx);
    digest_to_hex(digest, SHA1_SIZE, sha1);
    return string(sha1, SHA1_SIZE * 2);
}



static int32_t xfsc_file_check(const xfsc_file_t *file, enum XFSC_REQ_TYPE type)
{
    // check proto
    if (strcmp(file->proto, "http"))
        return XFSC_ERROR_PROTO_INVALID;

    // TODO! check ip and port

    // check prefix
    if (strcmp(file->prefix, "xfs"))
        return XFSC_ERROR_PREFIX_INVALID;

    // skip check buck

    // check filename
    // TODO! need check more
    switch(type) {
        case XFSC_REQ_TYPE_UPLOAD_FILE:
            break;
        case XFSC_REQ_TYPE_UPLOAD_BUFFER:
            break;
        case XFSC_REQ_TYPE_DELETE:
            if (file->filename == NULL)
                return XFSC_ERROR_FILENAME_INVALID;
            break;
        case XFSC_REQ_TYPE_HEAD:
            if (file->filename == NULL)
                return XFSC_ERROR_FILENAME_INVALID;
            break;
        case XFSC_REQ_TYPE_DOWNLOAD:
            if (file->filename == NULL)
                return XFSC_ERROR_FILENAME_INVALID;
            break;
        default:
            break;
    }

    return XFSC_ERROR_OK;
}



static int32_t xfsc_file_to_uri(string &uri, const xfsc_file_t *file, enum XFSC_REQ_TYPE type)
{
    int32_t ret = xfsc_file_check(file, type);
    if (ret != XFSC_ERROR_OK)
        return ret;

    int32_t len = 128 + strlen(file->filename);
    char *buf = (char *)malloc(len);
    if (!buf)
        return XFSC_ERROR_NO_MEMORY;

    string target;
    switch(type) {
        case XFSC_REQ_TYPE_UPLOAD_INFO:
        case XFSC_REQ_TYPE_HEAD:
            target = "fileinfo";
            break;
        case XFSC_REQ_TYPE_UPLOAD_FILE:
        case XFSC_REQ_TYPE_UPLOAD_BUFFER:
        case XFSC_REQ_TYPE_DELETE:
        case XFSC_REQ_TYPE_DOWNLOAD:
            target = "file";
            break;
        default:
            break;
    }

    snprintf(buf, len, "%s://%s:%d/%s?path=/%s/%s/%s", file->proto, file->ip,
            file->port, target.c_str(), file->prefix, file->buck,
            file->filename);
    uri = buf;
    free(buf);

    return XFSC_ERROR_OK;
}



static int32_t upload_buffer(CURL *curl, curl_user_data_t *userp, const char *uri, const char *buf, int64_t len, bool concurrent, bool open_debug=true)
{
    int32_t ret = XFSC_ERROR_OK;

    if (concurrent) {
        userp = new curl_user_data_t;
        userp->buf = (char *)malloc(1024);
        userp->pos = 0;
        userp->size = 1024;

        curl = curl_start();
    }

    int32_t code = curl_do(curl, userp, uri, XFSC_REQ_TYPE_UPLOAD_BUFFER, buf,
            len, open_debug);
    if (code != 200)
        ret = XFSC_ERROR_UP_FILE_FAIL;
    else {
        HttpParser parser;
        HttpParser::status_t status = parser.addBytes(userp->buf, userp->pos);
        if (status != HttpParser::Done)
            ret = XFSC_ERROR_UP_FILE_FAIL;
        else {
            // TODO! check location
            //const char *location = parser.getValue("location");
            const char *etag = parser.getValue("etag");
            //if (strcmp(uri, location))
            //    ret = XFSC_ERROR_UP_FILE_FAIL;
            if (strcmp(etag, sha1sum(buf, len).c_str()))
                ret = XFSC_ERROR_UP_FILE_FAIL;
        }
    }

    if (concurrent) {
        curl_stop(curl);

        free(userp->buf);
        delete userp;
    }

    return ret;
}



static int32_t do_upload_file(xfsc_t *xfsc, data_upload_file_t *data, CURL *curl, curl_user_data_t *userp)
{
    int32_t ret = XFSC_ERROR_OK;

    struct stat st;
    int fd = -1;

    if (stat(data->path.c_str(), &st)) {
        ret = XFSC_ERROR_FILE_STAT_FAIL;
        goto quit;
    }

    fd = open(data->path.c_str(), O_RDONLY, 0);
    if (fd == -1) {
        ret = XFSC_ERROR_FILE_OPEN_FAIL;
        goto quit;
    }

    if (st.st_size <= xfsc->config.max_needle_size) {
        char *buf = (char *)malloc(st.st_size);
        if (!buf) {
            ret = XFSC_ERROR_NO_MEMORY;
            goto quit;
        }

        read(fd, buf, st.st_size);  // FIXME! do loop read

        ret = upload_buffer(curl, userp, data->uri.c_str(), buf,
                st.st_size, false, xfsc->config.open_debug != 0);
        free(buf);
        if (ret != XFSC_ERROR_OK)
            goto quit;

        Json::Value root;
        root["location"] = data->name;
        xfsc->cbs.callback(data->arg, ret, root.toStyledString().c_str(), 0,
                0);
        return ret;
    }
    else {
        int64_t blocksize = xfsc->config.max_needle_size;

        char *buf = (char *)malloc(blocksize);
        if (!buf) {
            ret = XFSC_ERROR_NO_MEMORY;
            goto quit;
        }

        char *blockname = (char *)malloc(data->name.size() + 16);
        char *blockuri = (char *)malloc(data->uri.size() + 16);

        // upload all block
        vector<string> namelist;
        for (int64_t i = 0; i <= st.st_size / blocksize; ++i) {
            ssize_t nread = read(fd, buf, blocksize);  // FIXME! do loop read
            if (nread == 0 && i * blocksize == st.st_size)  // already read all
                break;

            sprintf(blockname, "%s_%06" PRId64, data->name.c_str(), i);
            sprintf(blockuri, "%s_%06" PRId64, data->uri.c_str(), i);
            namelist.push_back(blockname);

            ret = upload_buffer(curl, userp, blockuri, buf, nread, false,
                    xfsc->config.open_debug != 0);
            if (ret != XFSC_ERROR_OK)
                break;
        }
        free(blockname);
        free(blockuri);
        free(buf);

        if (ret == XFSC_ERROR_OK) {
            // upload meta block
            Json::Value root;
            Json::Value blocks;
            for (size_t i = 0; i != namelist.size(); ++i) {
                Json::Value block;
                block["filename"] = "/" + namelist[i];
                block["offset"] = (Json::Value::Int64)(i * blocksize);
                block["size"] = (Json::Value::Int64)
                        ((i == namelist.size() - 1) ?
                        st.st_size - i * blocksize : blocksize);
                blocks.append(block);
            }
            root["filename"] = data->name;
            root["filesize"] = (Json::Value::Int64)st.st_size;
            root["chunksize"] = (Json::Value::Int64)blocksize;
            root["Chunks"] = blocks;
            ret = upload_buffer(curl, userp, data->uri_info.c_str(),
                    root.toStyledString().c_str(), root.toStyledString().size(),
                    false, xfsc->config.open_debug != 0);

            if (ret == XFSC_ERROR_OK) {
                Json::Value root;
                root["location"] = data->name;
                xfsc->cbs.callback(data->arg, ret,
                        root.toStyledString().c_str(), 0, 0);
                return ret;
            }
        }
    }

quit:
    if (ret != XFSC_ERROR_OK)
        xfsc->cbs.callback(data->arg, ret, 0, 0, 0);
    if (fd >= 0)
        close(fd);
    return ret;
}



static int32_t do_delete_file(xfsc_t *xfsc, data_delete_t *data, CURL *curl, curl_user_data_t *userp)
{
    int32_t ret = XFSC_ERROR_OK;

    int32_t code = curl_do(curl, userp, data->uri.c_str(),
            XFSC_REQ_TYPE_DELETE, 0, 0, xfsc->config.open_debug != 0);
    if (code == 404)
        ret = XFSC_ERROR_FILE_NOT_FOUND;
    else if (code != 200)
        ret = XFSC_ERROR_DEL_FILE_FAIL;

    xfsc->cbs.callback(data->arg, ret, 0, 0, 0);
    return ret;
}



static int32_t do_head_file(xfsc_t *xfsc, data_head_t *data, CURL *curl, curl_user_data_t *userp)
{
    int32_t ret = XFSC_ERROR_OK;

    int32_t code = curl_do(curl, userp, data->uri.c_str(),
            XFSC_REQ_TYPE_HEAD, 0, 0, xfsc->config.open_debug != 0);
    if (code == 404)
        ret = XFSC_ERROR_FILE_NOT_FOUND;
    else if (code != 200)
        ret = XFSC_ERROR_INFO_FILE_FAIL;

    if (ret == XFSC_ERROR_OK) {
        Json::Value root;
        Json::Reader reader;
        Json::Value info;
        if (reader.parse(string(userp->buf, userp->pos), root)) {
            if (!root["Filename"].empty()) {
                info["location"] = root["Filename"];
                info["size"] = root["Filesize"];
            }
            else if (!root["Dir"].empty()) {
                info["location"] = root["Dir"];
                info["file"] = root["Files"];
                info["subdir"] = root["SubDirs"];
            }
            else {
                ret = XFSC_ERROR_INFO_FILE_FAIL;
            }
        }

        if (ret == XFSC_ERROR_OK) {
            xfsc->cbs.callback(data->arg, ret, info.toStyledString().c_str(), 0,
                    0);
            return ret;
        }
    }

    xfsc->cbs.callback(data->arg, ret, 0, 0, 0);
    return ret;
}



static int32_t do_download_file(xfsc_t *xfsc, data_download_t *data, CURL *curl, curl_user_data_t *userp)
{
    int32_t ret = XFSC_ERROR_OK;
    char *out = 0;
    int64_t size = 0;

    int64_t blocksize = xfsc->config.max_needle_size;

    int64_t start = data->offset;
    int64_t end = data->size ? data->offset + data->size : INT64_MAX;
    int64_t pos = start;

    char range[64];
    while (pos < end) {
        int64_t pos_next = (pos / blocksize + 1) * blocksize;
        pos_next = pos_next < end ? pos_next : end;
        sprintf(range, "%" PRId64 "-%" PRId64, pos, pos_next - 1);
        size_t len = strlen(range);

        int32_t code = curl_do(curl, userp, data->uri.c_str(),
                XFSC_REQ_TYPE_DOWNLOAD, range, len,
                xfsc->config.open_debug != 0);
        if (code == 404) {
            ret = XFSC_ERROR_FILE_NOT_FOUND;
            goto quit;
        }
        else if (code != 200) {
            ret = XFSC_ERROR_DOWN_FILE_FAIL;
            goto quit;
        }

        size += userp->pos;
        out = (char *)realloc(out, size);
        if (!out) {
            ret = XFSC_ERROR_NO_MEMORY;
            goto quit;
        }
        memcpy(out + size - userp->pos, userp->buf, userp->pos);

        if ((int64_t)userp->pos < pos_next - pos)
            break;

        pos = pos_next;
    }

    xfsc->cbs.callback(data->arg, ret, 0, out, size);

quit:
    if (out)
        free(out);
    if (ret != XFSC_ERROR_OK)
        xfsc->cbs.callback(data->arg, ret, 0, 0, 0);
    return ret;
}



static int32_t new_task(xfsc_t *xfsc, enum XFSC_REQ_TYPE type, void *data)
{
    data_common_t *data_common = new data_common_t;
    if (!data_common)
        return XFSC_ERROR_NO_MEMORY;

    data_common->type = type;
    data_common->data = data;

    return xqueue_en(xfsc->xqueue, data_common);
}



static void *xfsc_worker(void *arg)
{
    xfsc_t *xfsc = (xfsc_t *)arg;

    // get memory for curl
    curl_user_data_t *userp = new curl_user_data_t;
    userp->buf = (char *)malloc(1024);
    userp->pos = 0;
    userp->size = 1024;

    CURL *curl = curl_start();
    if (!curl)
        return 0;

    while (!xfsc->stop) {
        data_common_t *data_common = (data_common_t *)xqueue_de(xfsc->xqueue);
        if (!data_common)
            break;

        switch(data_common->type) {
            case XFSC_REQ_TYPE_UPLOAD_FILE:
                do_upload_file(xfsc, (data_upload_file_t *)data_common->data,
                        curl, userp);  // FIXME! need check ret
                break;
            case XFSC_REQ_TYPE_DELETE:
                do_delete_file(xfsc, (data_delete_t *)data_common->data, curl,
                        userp);  // FIXME! need check ret
                break;
            case XFSC_REQ_TYPE_HEAD:
                do_head_file(xfsc, (data_head_t *)data_common->data, curl,
                        userp);  // FIXME! need check ret
                break;
            case XFSC_REQ_TYPE_DOWNLOAD:
                do_download_file(xfsc, (data_download_t *)data_common->data,
                        curl, userp);
                break;
            default:
                break;
        }

        delete data_common;
    }

    curl_stop(curl);

    // free memory for curl
    free(userp->buf);
    delete userp;

    return 0;
}



extern "C" const char *xfsc_perror(enum XFSC_ERROR err)
{
    switch(err) {
        case XFSC_ERROR_OK:
            return "No error";
        case XFSC_ERROR_NO_MEMORY:
            return "No enough memory";
        case XFSC_ERROR_PARAM_INVALID:
            return "Parameters invalid";
        case XFSC_ERROR_PROTO_INVALID:
            return "Protocol invalid";
        case XFSC_ERROR_PREFIX_INVALID:
            return "Prefix invalid";
        case XFSC_ERROR_FILENAME_INVALID:
            return "Filename invalid";
        case XFSC_ERROR_UP_FILE_FAIL:
            return "Upload file fail";
        case XFSC_ERROR_FILE_STAT_FAIL:
            return "Local file stat fail";
        case XFSC_ERROR_FILE_OPEN_FAIL:
            return "Local file open fail";
        case XFSC_ERROR_DEL_FILE_FAIL:
            return "Delete file fail";
        case XFSC_ERROR_INFO_FILE_FAIL:
            return "Get file info fail";
        case XFSC_ERROR_DOWN_FILE_FAIL:
            return "Download file fail";
        case XFSC_ERROR_FILE_NOT_FOUND:
            return "File not found, 404";
        default:
            return "Unknown error";
    }
}



extern "C" int32_t xfsc_init(xfsc_t **xfsc, xfsc_config_t config, xfsc_callbacks_t cbs)
{
    if (!xfsc || !config.thread_num || !config.max_needle_size || !cbs.callback
            || !cbs.callback_log)
        return XFSC_ERROR_PARAM_INVALID;

    *xfsc = new xfsc_t;
    (*xfsc)->config = config;
    (*xfsc)->cbs = cbs;
    (*xfsc)->xqueue = xqueue_create();
    (*xfsc)->ylog = ylog_open("xfsc", 1, 1, 1, cbs.callback_log);
    (*xfsc)->stop = false;

    for (int32_t i = 0; i != (*xfsc)->config.thread_num; ++i) {
        pthread_t tid;
        pthread_create(&tid, 0, xfsc_worker, *xfsc);
        pthread_detach(tid);
    }

    return XFSC_ERROR_OK;
}



extern "C" int32_t xfsc_destroy(xfsc_t *xfsc)
{
    if (!xfsc)
        return XFSC_ERROR_PARAM_INVALID;

    // TODO! finish tasks

    xfsc->stop = true;
    xqueue_destroy(xfsc->xqueue);  // TODO! incorrect
    ylog_close(xfsc->ylog);
    delete xfsc;

    return XFSC_ERROR_OK;
}



extern "C" int32_t xfsc_upload_file(xfsc_t *xfsc, void *arg, const xfsc_file_t *file, const char *path)
{
    if (!xfsc || !path)
        return XFSC_ERROR_PARAM_INVALID;

    string uri, uri_info;
    int32_t ret = xfsc_file_to_uri(uri, file, XFSC_REQ_TYPE_UPLOAD_FILE);
    if (ret != XFSC_ERROR_OK)
        return ret;
    ret = xfsc_file_to_uri(uri_info, file, XFSC_REQ_TYPE_UPLOAD_INFO);
    if (ret != XFSC_ERROR_OK)
        return ret;

    data_upload_file_t *data = new data_upload_file_t;
    data->arg = arg;
    data->uri = uri;
    data->uri_info = uri_info;
    data->path = path;
    data->name = file->filename;

    return new_task(xfsc, XFSC_REQ_TYPE_UPLOAD_FILE, data);
}



extern "C" int32_t xfsc_delete(xfsc_t *xfsc, void *arg, const xfsc_file_t *file)
{
    if (!xfsc)
        return XFSC_ERROR_PARAM_INVALID;

    string uri;
    int32_t ret = xfsc_file_to_uri(uri, file, XFSC_REQ_TYPE_DELETE);
    if (ret != XFSC_ERROR_OK)
        return ret;

    data_delete_t *data = new data_delete_t;
    data->arg = arg;
    data->uri = uri;

    return new_task(xfsc, XFSC_REQ_TYPE_DELETE, data);
}



extern "C" int32_t xfsc_info(xfsc_t *xfsc, void *arg, const xfsc_file_t *file)
{
    if (!xfsc)
        return XFSC_ERROR_PARAM_INVALID;

    string uri;
    int32_t ret = xfsc_file_to_uri(uri, file, XFSC_REQ_TYPE_HEAD);
    if (ret != XFSC_ERROR_OK)
        return ret;

    data_head_t *data = new data_head_t;
    data->arg = arg;
    data->uri = uri;

    return new_task(xfsc, XFSC_REQ_TYPE_HEAD, data);
}



extern "C" int32_t xfsc_download(xfsc_t *xfsc, void *arg, const xfsc_file_t *file, int64_t offset, int64_t size)
{
    if (!xfsc || offset < 0 || size < 0)
        return XFSC_ERROR_PARAM_INVALID;

    string uri;
    int32_t ret = xfsc_file_to_uri(uri, file, XFSC_REQ_TYPE_DOWNLOAD);
    if (ret != XFSC_ERROR_OK)
        return ret;

    data_download_t *data = new data_download_t;
    data->arg = arg;
    data->uri = uri;
    data->offset = offset;
    data->size = size;

    return new_task(xfsc, XFSC_REQ_TYPE_DOWNLOAD, data);
}
