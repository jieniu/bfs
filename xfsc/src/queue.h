#pragma once

#include <stdint.h>


typedef struct _xqueue_s xqueue_t;


xqueue_t *xqueue_create();
int32_t   xqueue_destroy(xqueue_t *xqueue);
int32_t   xqueue_en(xqueue_t *xqueue, void *node);
void     *xqueue_de(xqueue_t *xqueue);
int32_t   xqueue_num(xqueue_t *xqueue);
