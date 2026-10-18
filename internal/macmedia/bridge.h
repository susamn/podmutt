#include <stdint.h>
int pm_prepare(void);
void pm_run(void);
void pm_stop(void);
void *pm_install(uintptr_t handle);
int pm_update(void *controller, const char *title, double position, double duration, int playing, int current, int next, int seekable);
void pm_close(void *controller);
#ifdef PODMUTT_NATIVE_CHECK
// Diagnostic symbols exist only with the nativecheck build tag. No private
// MediaPlayer sendCommand API is used; invoke the actual registered block.
void *pm_check_capture(void *controller, int action);
int pm_check_fire(void *handler);
void pm_check_release(void *handler);
int pm_check_controls(void);
int pm_check_state(void);
char *pm_check_title(void);
#endif
