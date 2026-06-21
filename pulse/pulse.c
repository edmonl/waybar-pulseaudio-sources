#include "pulse.h"

#include <stdlib.h>
#include <string.h>

static pulse_error_t pulse_error_ok() {
  pulse_error_t error;
  error.code = PULSE_ERROR_NONE;
  error.pa_errno = 0;
  return error;
}

static pulse_error_t pulse_error(pulse_error_code_t code, int pa_errno) {
  pulse_error_t error;
  error.code = code;
  error.pa_errno = pa_errno;
  return error;
}

static bool pulse_error_failed(pulse_error_t error) {
  return error.code != PULSE_ERROR_NONE;
}

/* client */
struct pulse_client {
  pa_threaded_mainloop *mainloop;
  pa_context *context;
  uint64_t event_generation;
  bool mainloop_started;
};

pulse_client_t *pulse_client_new(void) {
  return calloc(1, sizeof(pulse_client_t));
}

void pulse_client_free(pulse_client_t *client) {
  if (!client) {
    return;
  }

  /* A context is only created after mainloop creation succeeds. */
  if (client->context) {
    pa_threaded_mainloop_lock(client->mainloop);
    pa_context_set_state_callback(client->context, NULL, NULL);
    pa_context_set_subscribe_callback(client->context, NULL, NULL);
    pa_context_disconnect(client->context);
    pa_context_unref(client->context);
    client->context = NULL;
    pa_threaded_mainloop_unlock(client->mainloop);
  }
  if (client->mainloop_started) {
    pa_threaded_mainloop_stop(client->mainloop);
    client->mainloop_started = false;
  }
  if (client->mainloop) {
    pa_threaded_mainloop_free(client->mainloop);
  }

  free(client);
}

static void pulse_source_free_fields(pulse_source_t *source) {
  free(source->name);
  free(source->description);
}

void pulse_source_free(pulse_source_t *source) {
  if (source) {
    pulse_source_free_fields(source);
    free(source);
  }
}

static bool pulse_source_copy(pulse_source_t *source,
                              const pa_source_info *info) {
  pulse_source_free_fields(source);
  memset(source, 0, sizeof(*source));
  source->index = info->index;

  source->name = strdup(info->name);
  if (!source->name) {
    return false;
  }

  const char *description = info->description ? info->description : info->name;
  source->description = strdup(description);
  if (!source->description) {
    free(source->name);
    source->name = NULL;
    return false;
  }

  pa_volume_t average = pa_cvolume_avg(&info->volume);
  source->volume_percent =
      (int)(((uint64_t)average * 100 + PA_VOLUME_NORM / 2) / PA_VOLUME_NORM);
  source->mute = info->mute != 0;

  return true;
}

void pulse_snapshot_free(pulse_snapshot_t *snapshot) {
  if (!snapshot) {
    return;
  }

  if (snapshot->sources) {
    for (int i = 0; i < snapshot->count; ++i) {
      pulse_source_free_fields(&snapshot->sources[i]);
    }
    free(snapshot->sources);
  }
  free(snapshot->default_source_name);

  free(snapshot);
}

/* helpers */
typedef struct {
  pulse_client_t *client;
  bool success;
} success_data_t;

static void pulse_success_cb(pa_context *context, int success, void *userdata) {
  (void)context;

  success_data_t *data = (success_data_t *)userdata;
  data->success = success != 0;
  pa_threaded_mainloop_signal(data->client->mainloop, 0);
}

static pulse_error_t pulse_context_ready(pulse_client_t *client) {
  pa_context_state_t state = pa_context_get_state(client->context);
  if (state != PA_CONTEXT_READY) {
    return pulse_error(PULSE_ERROR_CONTEXT_NOT_READY,
                       pa_context_errno(client->context));
  }
  return pulse_error_ok();
}

static bool pulse_source_is_monitor(const char *name) {
  size_t name_len = strlen(name);
  return name_len >= 8 && strcmp(name + name_len - 8, ".monitor") == 0;
}

static bool ignore_source(const pa_source_info *source) {
  return !source || !source->name || pulse_source_is_monitor(source->name);
}

static pulse_error_t pulse_wait_for_operation(pulse_client_t *client,
                                              pa_operation *operation) {
  if (!operation) {
    return pulse_error(PULSE_ERROR_OPERATION_START, 0);
  }

  while (pa_operation_get_state(operation) == PA_OPERATION_RUNNING) {
    pa_threaded_mainloop_wait(client->mainloop);
    pulse_error_t error = pulse_context_ready(client);
    if (pulse_error_failed(error)) {
      pa_operation_cancel(operation);
      pa_operation_unref(operation);
      return error;
    }
  }

  pa_operation_state_t state = pa_operation_get_state(operation);
  pa_operation_unref(operation);

  if (state != PA_OPERATION_DONE) {
    return pulse_error(PULSE_ERROR_OPERATION_CANCELLED, 0);
  }

  return pulse_error_ok();
}

/* connect */
static void pulse_context_state_cb(pa_context *context, void *userdata) {
  (void)context;

  pulse_client_t *client = (pulse_client_t *)userdata;
  pa_threaded_mainloop_signal(client->mainloop, 0);
}

pulse_error_t pulse_client_connect(pulse_client_t *client) {
  client->mainloop = pa_threaded_mainloop_new();
  if (!client->mainloop) {
    return pulse_error(PULSE_ERROR_MAINLOOP_NEW, 0);
  }

  client->context =
      pa_context_new(pa_threaded_mainloop_get_api(client->mainloop),
                     "waybar-pulseaudio-sources");
  if (!client->context) {
    return pulse_error(PULSE_ERROR_CONTEXT_NEW, 0);
  }

  pa_context_set_state_callback(client->context, pulse_context_state_cb,
                                client);

  pa_threaded_mainloop_lock(client->mainloop);

  if (pa_context_connect(client->context, NULL, PA_CONTEXT_NOFLAGS, NULL) < 0) {
    pulse_error_t error = pulse_error(PULSE_ERROR_CONTEXT_CONNECT,
                                      pa_context_errno(client->context));
    pa_threaded_mainloop_unlock(client->mainloop);
    return error;
  }

  if (pa_threaded_mainloop_start(client->mainloop) < 0) {
    pulse_error_t error = pulse_error(PULSE_ERROR_MAINLOOP_START,
                                      pa_context_errno(client->context));
    pa_threaded_mainloop_unlock(client->mainloop);
    return error;
  }
  client->mainloop_started = true;

  for (;;) {
    pa_context_state_t state = pa_context_get_state(client->context);
    if (state == PA_CONTEXT_READY) {
      pa_threaded_mainloop_unlock(client->mainloop);
      return pulse_error_ok();
    }
    if (!PA_CONTEXT_IS_GOOD(state)) {
      pulse_error_t error = pulse_error(PULSE_ERROR_CONTEXT_FAILED,
                                        pa_context_errno(client->context));
      pa_threaded_mainloop_unlock(client->mainloop);
      return error;
    }
    pa_threaded_mainloop_wait(client->mainloop);
  }
}

/* server info */
typedef struct {
  pulse_client_t *client;
  char *default_source_name;
  bool failed;
} server_info_data_t;

static void pulse_server_info_cb(pa_context *context,
                                 const pa_server_info *info, void *userdata) {
  (void)context;

  server_info_data_t *data = (server_info_data_t *)userdata;
  if (info && info->default_source_name) {
    data->default_source_name = strdup(info->default_source_name);
  }

  if (!data->default_source_name) {
    data->failed = true;
  }

  pa_threaded_mainloop_signal(data->client->mainloop, 0);
}

static char *pulse_get_default_source_name(pulse_client_t *client,
                                           pulse_error_t *error) {
  server_info_data_t data;
  data.client = client;
  data.default_source_name = NULL;
  data.failed = false;

  pa_operation *operation =
      pa_context_get_server_info(client->context, pulse_server_info_cb, &data);
  *error = pulse_wait_for_operation(client, operation);
  if (pulse_error_failed(*error)) {
    free(data.default_source_name);
    return NULL;
  }
  if (data.failed) {
    free(data.default_source_name);
    *error = pulse_error(PULSE_ERROR_SERVER_INFO, 0);
    return NULL;
  }

  return data.default_source_name;
}

/* get sources */
typedef struct {
  pulse_client_t *client;
  pulse_snapshot_t *snapshot;
  bool failed;
} snapshot_data_t;

static void pulse_source_info_cb(pa_context *context,
                                 const pa_source_info *info, int eol,
                                 void *userdata) {
  (void)context;

  snapshot_data_t *data = (snapshot_data_t *)userdata;
  if (eol < 0) {
    data->failed = true;
    pa_threaded_mainloop_signal(data->client->mainloop, 0);
    return;
  }

  if (eol > 0) {
    pa_threaded_mainloop_signal(data->client->mainloop, 0);
    return;
  }

  if (data->failed) {
    return;
  }

  if (ignore_source(info)) {
    return;
  }

  pulse_snapshot_t *snapshot = data->snapshot;
  int next_count = snapshot->count + 1;
  pulse_source_t *sources =
      realloc(snapshot->sources, sizeof(pulse_source_t) * next_count);
  if (!sources) {
    data->failed = true;
    pa_threaded_mainloop_signal(data->client->mainloop, 0);
    return;
  }

  snapshot->sources = sources;
  pulse_source_t *source = &snapshot->sources[snapshot->count];
  memset(source, 0, sizeof(*source));
  if (!pulse_source_copy(source, info)) {
    data->failed = true;
    pa_threaded_mainloop_signal(data->client->mainloop, 0);
    return;
  }

  snapshot->count = next_count;
}

pulse_snapshot_t *pulse_get_sources(pulse_client_t *client,
                                    pulse_error_t *error) {
  pulse_snapshot_t *snapshot = calloc(1, sizeof(pulse_snapshot_t));
  if (!snapshot) {
    *error = pulse_error(PULSE_ERROR_SNAPSHOT_ALLOC, 0);
    return NULL;
  }

  snapshot_data_t data;
  data.client = client;
  data.snapshot = snapshot;
  data.failed = false;

  pa_threaded_mainloop_lock(client->mainloop);

  *error = pulse_context_ready(client);
  if (pulse_error_failed(*error)) {
    pa_threaded_mainloop_unlock(client->mainloop);
    pulse_snapshot_free(snapshot);
    return NULL;
  }

  snapshot->default_source_name = pulse_get_default_source_name(client, error);
  if (!snapshot->default_source_name) {
    pa_threaded_mainloop_unlock(client->mainloop);
    pulse_snapshot_free(snapshot);
    return NULL;
  }

  pa_operation *operation = pa_context_get_source_info_list(
      client->context, pulse_source_info_cb, &data);
  *error = pulse_wait_for_operation(client, operation);
  pa_threaded_mainloop_unlock(client->mainloop);
  if (pulse_error_failed(*error)) {
    pulse_snapshot_free(snapshot);
    return NULL;
  }
  if (data.failed) {
    *error = pulse_error(PULSE_ERROR_SOURCE_LIST, 0);
    pulse_snapshot_free(snapshot);
    return NULL;
  }

  return snapshot;
}

/* get default source */
typedef struct {
  pulse_client_t *client;
  pulse_source_t *source;
  bool failed;
} default_source_data_t;

static void pulse_default_source_info_cb(pa_context *context,
                                         const pa_source_info *info, int eol,
                                         void *userdata) {
  (void)context;

  default_source_data_t *data = (default_source_data_t *)userdata;
  if (eol < 0) {
    data->failed = true;
    pa_threaded_mainloop_signal(data->client->mainloop, 0);
    return;
  }

  if (eol > 0) {
    pa_threaded_mainloop_signal(data->client->mainloop, 0);
    return;
  }

  if (data->failed) {
    return;
  }

  if (ignore_source(info)) {
    return;
  }

  if (!data->source) {
    data->source = calloc(1, sizeof(pulse_source_t));
    if (!data->source) {
      data->failed = true;
      pa_threaded_mainloop_signal(data->client->mainloop, 0);
      return;
    }
  }

  if (!pulse_source_copy(data->source, info)) {
    pulse_source_free(data->source);
    data->source = NULL;
    data->failed = true;
    pa_threaded_mainloop_signal(data->client->mainloop, 0);
  }
}

pulse_source_t *pulse_get_default_source(pulse_client_t *client,
                                         pulse_error_t *error) {
  default_source_data_t data;
  data.client = client;
  data.source = NULL;
  data.failed = false;

  pa_threaded_mainloop_lock(client->mainloop);

  *error = pulse_context_ready(client);
  if (pulse_error_failed(*error)) {
    pa_threaded_mainloop_unlock(client->mainloop);
    return NULL;
  }

  char *default_source_name = pulse_get_default_source_name(client, error);
  if (!default_source_name) {
    pa_threaded_mainloop_unlock(client->mainloop);
    return NULL;
  }
  if (pulse_source_is_monitor(default_source_name)) {
    pa_threaded_mainloop_unlock(client->mainloop);
    free(default_source_name);
    return NULL;
  }

  pa_operation *operation =
      pa_context_get_source_info_by_name(client->context, default_source_name,
                                         pulse_default_source_info_cb, &data);
  *error = pulse_wait_for_operation(client, operation);
  pa_threaded_mainloop_unlock(client->mainloop);
  free(default_source_name);
  if (pulse_error_failed(*error)) {
    pulse_source_free(data.source);
    return NULL;
  }
  if (data.failed) {
    *error = pulse_error(PULSE_ERROR_DEFAULT_SOURCE, 0);
    pulse_source_free(data.source);
    return NULL;
  }

  return data.source;
}

/* set default source */
pulse_error_t pulse_set_default_source(pulse_client_t *client,
                                       const char *name) {
  success_data_t data;
  data.client = client;
  data.success = false;

  pa_threaded_mainloop_lock(client->mainloop);

  pulse_error_t error = pulse_context_ready(client);
  if (pulse_error_failed(error)) {
    pa_threaded_mainloop_unlock(client->mainloop);
    return error;
  }

  pa_operation *operation = pa_context_set_default_source(
      client->context, name, pulse_success_cb, &data);
  error = pulse_wait_for_operation(client, operation);
  pa_threaded_mainloop_unlock(client->mainloop);
  if (pulse_error_failed(error)) {
    return error;
  }
  if (!data.success) {
    return pulse_error(PULSE_ERROR_SET_DEFAULT_SOURCE, 0);
  }

  return pulse_error_ok();
}


/* subscribe */
static void pulse_subscribe_cb(pa_context *context,
                               pa_subscription_event_type_t event_type,
                               uint32_t index, void *userdata) {
  (void)context;
  (void)event_type;
  (void)index;

  pulse_client_t *client = (pulse_client_t *)userdata;
  ++client->event_generation;
  pa_threaded_mainloop_signal(client->mainloop, 0);
}

pulse_error_t pulse_client_subscribe(pulse_client_t *client) {
  success_data_t data;
  data.client = client;
  data.success = false;

  pa_threaded_mainloop_lock(client->mainloop);

  pulse_error_t error = pulse_context_ready(client);
  if (pulse_error_failed(error)) {
    pa_threaded_mainloop_unlock(client->mainloop);
    return error;
  }

  pa_context_set_subscribe_callback(client->context, pulse_subscribe_cb,
                                    client);
  pa_operation *operation = pa_context_subscribe(
      client->context,
      PA_SUBSCRIPTION_MASK_SOURCE | PA_SUBSCRIPTION_MASK_SERVER,
      pulse_success_cb, &data);
  error = pulse_wait_for_operation(client, operation);
  pa_threaded_mainloop_unlock(client->mainloop);
  if (pulse_error_failed(error)) {
    return error;
  }
  if (!data.success) {
    return pulse_error(PULSE_ERROR_SUBSCRIBE,
                       pa_context_errno(client->context));
  }

  return pulse_error_ok();
}

/* wait and wake up */
pulse_error_t pulse_wait_for_change(pulse_client_t *client) {
  pa_threaded_mainloop_lock(client->mainloop);

  pulse_error_t error = pulse_context_ready(client);
  if (pulse_error_failed(error)) {
    pa_threaded_mainloop_unlock(client->mainloop);
    return error;
  }

  uint64_t observed_generation = client->event_generation;
  while (client->event_generation == observed_generation) {
    pa_threaded_mainloop_wait(client->mainloop);
    error = pulse_context_ready(client);
    if (pulse_error_failed(error)) {
      pa_threaded_mainloop_unlock(client->mainloop);
      return error;
    }
  }

  pa_threaded_mainloop_unlock(client->mainloop);
  return pulse_error_ok();
}

void pulse_wakeup(pulse_client_t *client) {
  pa_threaded_mainloop_lock(client->mainloop);
  client->event_generation++;
  pa_threaded_mainloop_signal(client->mainloop, 0);
  pa_threaded_mainloop_unlock(client->mainloop);
}
