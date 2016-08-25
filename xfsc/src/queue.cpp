#include "queue.h"

#include <list>
#include <unistd.h>
#include <pthread.h>


using namespace std;


struct _xqueue_s {
    list<void *> nodes;
    pthread_mutex_t mu;
    pthread_cond_t cv;
};



xqueue_t *xqueue_create()
{
    xqueue_t *xqueue = new xqueue_t;
    pthread_mutex_init(&xqueue->mu, NULL);
    pthread_cond_init(&xqueue->cv, NULL);

    return xqueue;
}



int32_t xqueue_destroy(xqueue_t *xqueue)
{
    if (xqueue->nodes.size())
        return 1;

    pthread_cond_broadcast(&xqueue->cv);

    sleep(1);
    pthread_mutex_destroy(&xqueue->mu);
    pthread_cond_destroy(&xqueue->cv);
    delete xqueue;

    return 0;
}



int32_t xqueue_en(xqueue_t *xqueue, void *node)
{
    int32_t ret = 0;

    pthread_mutex_lock(&xqueue->mu);

    xqueue->nodes.push_back(node);
    ret = xqueue->nodes.size();

    pthread_mutex_unlock(&xqueue->mu);
    pthread_cond_signal(&xqueue->cv);

    return ret;
}



void *xqueue_de(xqueue_t *xqueue)
{
    void *node = 0;

    // if using C++11, just:
    // unique_lock<mutex> loc(xqueue->mu);
    // xqueue->cv.wait(loc);
    // no need unlock mu

    pthread_mutex_lock(&xqueue->mu);

    while (xqueue->nodes.empty())
        pthread_cond_wait(&xqueue->cv, &xqueue->mu);

    if (!xqueue->nodes.empty()) {
        node = *xqueue->nodes.begin();
        xqueue->nodes.pop_front();
    }

    pthread_mutex_unlock(&xqueue->mu);

    return node;
}



int32_t xqueue_num(xqueue_t *xqueue)
{
    if (xqueue)
        return xqueue->nodes.size();
    return -1;
}
