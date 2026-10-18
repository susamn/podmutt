#include <stdint.h>
void pm_prepare(void);
void pm_run(void);
void pm_stop(void);
void *pm_install(uintptr_t handle);
void pm_update(void *controller, const char *title, double position, double duration, int playing, int current, int next);
void pm_close(void *controller);
