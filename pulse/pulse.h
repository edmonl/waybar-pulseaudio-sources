#ifndef WAYBAR_PULSEAUDIO_SOURCES_PULSE_H
#define WAYBAR_PULSEAUDIO_SOURCES_PULSE_H

#include <pulse/pulseaudio.h>
#include <stdbool.h>
#include <stdint.h>

typedef enum {
  PULSE_ERROR_NONE = 0,
  PULSE_ERROR_CONTEXT_CONNECT,
  PULSE_ERROR_CONTEXT_FAILED,
  PULSE_ERROR_CONTEXT_NOT_READY,
  PULSE_ERROR_OPERATION_START,
  PULSE_ERROR_OPERATION_CANCELLED,
  PULSE_ERROR_SUBSCRIBE,
  PULSE_ERROR_SNAPSHOT_ALLOC,
  PULSE_ERROR_SERVER_INFO,
  PULSE_ERROR_SOURCE_LIST,
  PULSE_ERROR_NO_SOURCES,
  PULSE_ERROR_DEFAULT_SOURCE,
  PULSE_ERROR_SET_DEFAULT_SOURCE,
  PULSE_ERROR_CLIENT_SHUTDOWN
} pulse_error_code_t;

typedef struct {
  pulse_error_code_t code;
  int pa_errno;
} pulse_error_t;

typedef struct pulse_client pulse_client_t;

typedef struct {
  uint32_t index;
  char *name;
  char *description;
  int volume_percent;
  bool mute;
} pulse_source_t;

pulse_client_t *pulse_client_new(void);
void pulse_client_free(pulse_client_t *client);
pulse_error_t pulse_client_start(pulse_client_t *client);

void pulse_source_free(pulse_source_t *source);

pulse_source_t *pulse_get_default_source(pulse_client_t *client,
                                         pulse_error_t *error);
pulse_error_t pulse_cycle_default_source(pulse_client_t *client);
pulse_error_t pulse_wait_for_change(pulse_client_t *client);

/*
 * Permanently shuts down the client and wakes its blocking operations.
 * No client operation may be started afterward. Shutdown does not free the
 * client; active operations must return before pulse_client_free() is called.
 */
void pulse_client_shutdown(pulse_client_t *client);

#endif
