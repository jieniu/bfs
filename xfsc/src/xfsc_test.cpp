#include "xfsc.h"

#include <cstdio>
#include <unistd.h>
#include <ctime>


static time_t start_sec;


static void dummy_callback(void *arg, int32_t err, const char *json, const char *out, int64_t size)
{
    if (arg) {
        printf("arg=%s, err=%d, json='%s'\n", (char *)arg, err, json);
        if (out)
            printf("size of out: %lld\n", (long long signed)size);
    }
}


static void log_callback(const char*module, uint64_t millisecond, const char *msg)
{
    char buf[64];
    struct tm tm;
    time_t sec = millisecond / 1000 + start_sec;

    localtime_r(&sec, &tm);
    strftime(buf, sizeof buf, "%Y-%m-%d %H:%M:%S", &tm);
    snprintf(buf, sizeof buf, "%s.%03lld", buf,
            (long long signed)millisecond % 1000);
    printf("[%8s][%s] %s", module, buf, msg);
}


int main()
{
    start_sec = time(0);

    xfsc_config_t config;
    config.thread_num = 8;
    config.max_needle_size = (40*1024*1024);
    config.open_debug = 1;

    xfsc_callbacks_t cbs;
    cbs.callback = dummy_callback;
    cbs.callback_log = log_callback;

    xfsc_t *xfsc;
    xfsc_init(&xfsc, config, cbs);
    sleep(1);

    xfsc_file_t file = {"http", "10.10.191.8", 2232, "xfs", "test", 0};
    for (int i = 0; i != 1; ++i) {
        file.filename = "xx/0001.jpg";
        xfsc_upload_file(xfsc, (void *)"U", &file, "/tmp/0001.jpg");

        file.filename = "xx/0001.jpg";
        xfsc_download(xfsc, (void *)"G", &file, 0, 0);

        file.filename = "xx/0001.jpg";
        xfsc_delete(xfsc, (void *)"D", &file);

        file.filename = "xx/";
        xfsc_info(xfsc, (void *)"H", &file);
    }

    pause();
    xfsc_destroy(xfsc);

    return 0;
}
