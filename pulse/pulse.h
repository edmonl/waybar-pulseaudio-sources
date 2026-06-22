#ifndef WAYBAR_PULSEAUDIO_SOURCES_PULSE_H
#define WAYBAR_PULSEAUDIO_SOURCES_PULSE_H

#include <pulse/pulseaudio.h>
#include <stdbool.h>
#include <stdint.h>

typedef enum {
  PULSE_ERROR_NONE = 0,
  PULSE_ERROR_CONTEXT_NEW,
  PULSE_ERROR_CONTEXT_CONNECT,
  PULSE_ERROR_MAINLOOP_START,
  PULSE_ERROR_CONTEXT_FAILED,
  PULSE_ERROR_CONTEXT_NOT_READY,
  PULSE_ERROR_OPERATION_START,
  PULSE_ERROR_OPERATION_CANCELLED,
  PULSE_ERROR_SUBSCRIBE,
  PULSE_ERROR_SNAPSHOT_ALLOC,
  PULSE_ERROR_SERVER_INFO,
  PULSE_ERROR_SOURCE_LIST,
  PULSE_ERROR_DEFAULT_SOURCE,
  PULSE_ERROR_SET_DEFAULT_SOURCE,
  PULSE_ERROR_CLIENT_CANCELLED
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

typedef struct {
  pulse_source_t *sources;
  char *default_source_name;
  int count;
} pulse_snapshot_t;

pulse_client_t *pulse_client_new(void);
void pulse_client_free(pulse_client_t *client);
pulse_error_t pulse_client_connect(pulse_client_t *client);
pulse_error_t pulse_client_subscribe(pulse_client_t *client);

void pulse_source_free(pulse_source_t *source);
void pulse_snapshot_free(pulse_snapshot_t *snapshot);

pulse_source_t *pulse_get_default_source(pulse_client_t *client,
                                         pulse_error_t *error);
pulse_snapshot_t *pulse_get_sources(pulse_client_t *client,
                                    pulse_error_t *error);
pulse_error_t pulse_set_default_source(pulse_client_t *client,
                                       const char *name);
pulse_error_t pulse_wait_for_change(pulse_client_t *client);
void pulse_client_cancel(pulse_client_t *client);

#endif
