#include "pulse.h"

#include <pulse/context.h>
#include <pulse/def.h>
#include <pulse/thread-mainloop.h>
#include <stdlib.h>
#include <string.h>

/**
 * client
 */
struct pulse_client {
  pa_threaded_mainloop *mainloop;
  pa_context *context;
  bool event_pending;
  /* Shutdown is terminal; this flag is never cleared. */
  bool shutdown;
};

/**
 * errors
 */
static pulse_error_t error_ok(void) {
  pulse_error_t error;
  error.code = PULSE_ERROR_NONE;
  error.pa_errno = 0;
  return error;
}

static pulse_error_t error_with_code(pulse_error_code_t code,
                                     pulse_client_t *client_error_src) {
  pulse_error_t error;
  error.code = code;
  error.pa_errno = 0;
  if (client_error_src) {
    error.pa_errno = pa_context_errno(client_error_src->context);
  }
  return error;
}

static bool error_failed(pulse_error_t error) {
  return error.code != PULSE_ERROR_NONE;
}

/**
 * client: success helper
 */
typedef struct {
  pulse_client_t *client;
  bool success;
} success_data_t;

static void success_cb(pa_context *context, int success, void *userdata) {
  (void)context;

  success_data_t *data = (success_data_t *)userdata;
  data->success = success != 0;
  pa_threaded_mainloop_signal(data->client->mainloop, 0);
}

/**
 * client: helpers
 */
static pulse_error_t client_running(pulse_client_t *client) {
  if (client->shutdown) {
    return error_with_code(PULSE_ERROR_CLIENT_SHUTDOWN, NULL);
  }
  return error_ok();
}

static pulse_error_t client_ready(pulse_client_t *client) {
  pulse_error_t error = client_running(client);
  if (error_failed(error)) {
    return error;
  }

  pa_context_state_t state = pa_context_get_state(client->context);
  if (state == PA_CONTEXT_READY) {
    return error_ok();
  }

  return error_with_code(PULSE_ERROR_CONTEXT_NOT_READY, client);
}

static pulse_error_t
client_with_lock(pulse_client_t *client,
                 pulse_error_t (*func)(pulse_client_t *client)) {
  pa_threaded_mainloop_lock(client->mainloop);
  pulse_error_t error = func(client);
  pa_threaded_mainloop_unlock(client->mainloop);
  return error;
}

static pulse_error_t
client_lock_ready(pulse_client_t *client,
                  pulse_error_t (*func)(pulse_client_t *client, void *data),
                  void *data) {
  pa_threaded_mainloop_lock(client->mainloop);
  pulse_error_t error = client_ready(client);
  if (!error_failed(error)) {
    error = func(client, data);
  }
  pa_threaded_mainloop_unlock(client->mainloop);
  return error;
}

static pulse_error_t client_wait_ready(pulse_client_t *client) {
  pa_threaded_mainloop_wait(client->mainloop);
  return client_ready(client);
}

/**
 * client: wait for operation
 */
static pulse_error_t wait_for_operation(pulse_client_t *client,
                                        pa_operation *operation) {
  if (!operation) {
    return error_with_code(PULSE_ERROR_OPERATION_START, client);
  }

  pa_operation_state_t state = pa_operation_get_state(operation);
  while (state == PA_OPERATION_RUNNING) {
    pulse_error_t error = client_wait_ready(client);
    if (error_failed(error)) {
      pa_operation_cancel(operation);
      pa_operation_unref(operation);
      return error;
    }

    state = pa_operation_get_state(operation);
  }

  pa_operation_unref(operation);
  if (state != PA_OPERATION_DONE) {
    return error_with_code(PULSE_ERROR_OPERATION_CANCELLED, NULL);
  }

  return error_ok();
}

/**
 * client: connect
 */
static void context_state_cb(pa_context *context, void *userdata) {
  (void)context;

  pulse_client_t *client = (pulse_client_t *)userdata;
  pa_threaded_mainloop_signal(client->mainloop, 0);
}

static pulse_error_t client_connect(pulse_client_t *client) {
  if (pa_context_connect(client->context, NULL, PA_CONTEXT_NOFLAGS, NULL) < 0) {
    return error_with_code(PULSE_ERROR_CONTEXT_CONNECT, client);
  }

  pa_context_set_state_callback(client->context, context_state_cb, client);

  pulse_error_t error;
  do {
    pa_context_state_t state = pa_context_get_state(client->context);
    if (state == PA_CONTEXT_READY) {
      return error_ok();
    }

    if (!PA_CONTEXT_IS_GOOD(state)) {
      error = error_with_code(PULSE_ERROR_CONTEXT_FAILED, client);
      break;
    }

    pa_threaded_mainloop_wait(client->mainloop);
    error = client_running(client);
  } while (!error_failed(error));

  pa_context_disconnect(client->context);
  return error;
}

/**
 * client: subscribe
 */
static void subscribe_cb(pa_context *context,
                         pa_subscription_event_type_t event_type,
                         uint32_t index, void *userdata) {
  (void)context;
  (void)event_type;
  (void)index;

  pulse_client_t *client = (pulse_client_t *)userdata;
  client->event_pending = true;
  pa_threaded_mainloop_signal(client->mainloop, 0);
}

static pulse_error_t client_subscribe(pulse_client_t *client) {
  pa_context_set_subscribe_callback(client->context, subscribe_cb, client);

  success_data_t data;
  data.client = client;
  data.success = false;

  pulse_error_t error = wait_for_operation(
      client, pa_context_subscribe(client->context,
                                   PA_SUBSCRIPTION_MASK_SOURCE |
                                       PA_SUBSCRIPTION_MASK_SERVER,
                                   success_cb, &data));
  if (error_failed(error)) {
    pa_context_set_subscribe_callback(client->context, NULL, NULL);
    return error;
  }

  if (!data.success) {
    pa_context_set_subscribe_callback(client->context, NULL, NULL);
    return error_with_code(PULSE_ERROR_SUBSCRIBE, client);
  }

  return error_ok();
}

/**
 * client: start
 */
static pulse_error_t client_start(pulse_client_t *client) {
  pulse_error_t error = client_running(client);
  if (error_failed(error)) {
    return error;
  }

  error = client_connect(client);
  if (error_failed(error)) {
    return error;
  }

  error = client_subscribe(client);
  if (error_failed(error)) {
    pa_context_disconnect(client->context);
  }

  return error;
}

/**
 * source
 */
static bool source_is_monitor(const char *name) {
  size_t name_len = strlen(name);
  return name_len >= 8 && strcmp(name + name_len - 8, ".monitor") == 0;
}

static void source_free_fields(pulse_source_t *source) {
  free(source->name);
  free(source->description);
}

static bool source_copy(pulse_source_t *source, const pa_source_info *info) {
  source_free_fields(source);
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

/**
 * source: snapshot
 */
typedef struct {
  pulse_source_ref_t *sources;
  char *default_source_name;
  int count;
} pulse_snapshot_t;

typedef struct {
  pulse_client_t *client;
  pulse_snapshot_t *snapshot;
  bool failed;
} snapshot_data_t;

static void snapshot_free(pulse_snapshot_t *snapshot) {
  if (!snapshot) {
    return;
  }

  if (snapshot->sources) {
    for (int i = 0; i < snapshot->count; ++i) {
      free(snapshot->sources[i].name);
    }
    free(snapshot->sources);
  }
  free(snapshot->default_source_name);

  free(snapshot);
}

/**
 * source: server info
 */
typedef struct {
  pulse_client_t *client;
  char *default_source_name;
  bool success;
} server_info_data_t;

static void server_info_cb(pa_context *context, const pa_server_info *info,
                           void *userdata) {
  (void)context;

  server_info_data_t *data = (server_info_data_t *)userdata;
  if (info && info->default_source_name) {
    data->default_source_name = strdup(info->default_source_name);
  }

  if (data->default_source_name) {
    data->success = true;
  }

  pa_threaded_mainloop_signal(data->client->mainloop, 0);
}

static char *get_default_source_name(pulse_client_t *client,
                                     pulse_error_t *error) {
  server_info_data_t data;
  data.client = client;
  data.default_source_name = NULL;
  data.success = false;

  *error = wait_for_operation(
      client,
      pa_context_get_server_info(client->context, server_info_cb, &data));
  if (error_failed(*error)) {
    free(data.default_source_name);
    return NULL;
  }
  if (data.success) {
    return data.default_source_name;
  }

  free(data.default_source_name);
  *error = error_with_code(PULSE_ERROR_SERVER_INFO, NULL);
  return NULL;
}

/**
 * source: get sources
 */
static void source_info_cb(pa_context *context, const pa_source_info *info,
                           int eol, void *userdata) {
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

  if (!info || !info->name) {
    return;
  }

  pulse_snapshot_t *snapshot = data->snapshot;
  int next_count = snapshot->count + 1;
  pulse_source_ref_t *sources =
      realloc(snapshot->sources, sizeof(pulse_source_ref_t) * next_count);
  if (!sources) {
    data->failed = true;
    pa_threaded_mainloop_signal(data->client->mainloop, 0);
    return;
  }

  snapshot->sources = sources;
  pulse_source_ref_t *source = &snapshot->sources[snapshot->count];
  source->index = info->index;
  source->name = strdup(info->name);
  if (!source->name) {
    data->failed = true;
    pa_threaded_mainloop_signal(data->client->mainloop, 0);
    return;
  }

  snapshot->count = next_count;
}

static pulse_snapshot_t *get_sources(pulse_client_t *client,
                                     pulse_error_t *error) {
  pulse_snapshot_t *snapshot = calloc(1, sizeof(pulse_snapshot_t));
  if (!snapshot) {
    *error = error_with_code(PULSE_ERROR_SNAPSHOT_ALLOC, NULL);
    return NULL;
  }

  snapshot_data_t data;
  data.client = client;
  data.snapshot = snapshot;
  data.failed = false;

  snapshot->default_source_name = get_default_source_name(client, error);
  if (!snapshot->default_source_name) {
    snapshot_free(snapshot);
    return NULL;
  }

  *error = wait_for_operation(
      client,
      pa_context_get_source_info_list(client->context, source_info_cb, &data));
  if (error_failed(*error)) {
    snapshot_free(snapshot);
    return NULL;
  }
  if (data.failed) {
    *error = error_with_code(PULSE_ERROR_SOURCE_LIST, NULL);
    snapshot_free(snapshot);
    return NULL;
  }

  return snapshot;
}

/**
 * source: get default
 */
typedef struct {
  pulse_client_t *client;
  pulse_source_t *source;
  bool success;
} default_source_data_t;

static void default_source_info_cb(pa_context *context,
                                   const pa_source_info *info, int eol,
                                   void *userdata) {
  (void)context;

  default_source_data_t *data = (default_source_data_t *)userdata;
  if (eol) {
    pa_threaded_mainloop_signal(data->client->mainloop, 0);
    return;
  }

  if (!data->success && source_copy(data->source, info)) {
    data->success = true;
  }
}

static pulse_error_t get_default_source(pulse_client_t *client, void *out) {
  pulse_error_t error;
  char *default_source_name = get_default_source_name(client, &error);
  if (!default_source_name) {
    return error;
  }
  if (source_is_monitor(default_source_name)) {
    free(default_source_name);
    return error_with_code(PULSE_ERROR_NO_SOURCES, NULL);
  }

  default_source_data_t data;
  data.client = client;
  data.source = (pulse_source_t *)out;
  data.success = false;

  error = wait_for_operation(client, pa_context_get_source_info_by_name(
                                         client->context, default_source_name,
                                         default_source_info_cb, &data));
  free(default_source_name);
  if (error_failed(error)) {
    return error;
  }

  if (data.success) {
    return error_ok();
  }

  return error_with_code(PULSE_ERROR_DEFAULT_SOURCE, NULL);
}

/**
 * source: set default
 */
static pulse_error_t set_default_source(pulse_client_t *client,
                                        const char *name) {
  success_data_t data;
  data.client = client;
  data.success = false;

  pulse_error_t error = wait_for_operation(
      client,
      pa_context_set_default_source(client->context, name, success_cb, &data));
  if (error_failed(error)) {
    return error;
  }
  if (data.success) {
    return error_ok();
  }
  return error_with_code(PULSE_ERROR_SET_DEFAULT_SOURCE, NULL);
}

/**
 * wait
 */
static pulse_error_t wait_for_change(pulse_client_t *client, void *data) {
  (void)data;
  while (!client->event_pending) {
    pulse_error_t error = client_wait_ready(client);
    if (error_failed(error)) {
      return error;
    }
  }

  client->event_pending = false;
  return error_ok();
}

/**
 * next source
 */
const pulse_source_ref_t *
pulse_select_next_source(const pulse_source_ref_t *sources, int count,
                         const char *default_source_name) {
  const pulse_source_ref_t *first = NULL;
  const pulse_source_ref_t *current = NULL;
  for (int i = 0; i < count; ++i) {
    const pulse_source_ref_t *source = &sources[i];
    if (!source->name) {
      continue;
    }

    if (strcmp(source->name, default_source_name) == 0) {
      current = source;
    }

    if (source_is_monitor(source->name)) {
      continue;
    }
    if (!first || source->index < first->index) {
      first = source;
    }
  }

  if (!first) {
    return NULL;
  }

  if (!current) {
    return first;
  }

  const pulse_source_ref_t *next = NULL;
  for (int i = 0; i < count; ++i) {
    const pulse_source_ref_t *source = &sources[i];
    if (!source->name || source_is_monitor(source->name)) {
      continue;
    }
    if (source->index > current->index &&
        (!next || source->index < next->index)) {
      next = source;
    }
  }
  if (!next) {
    return first;
  }

  return next;
}

static pulse_error_t set_next_source_default(pulse_client_t *client,
                                             void *data) {
  (void)data;

  pulse_error_t error;
  pulse_snapshot_t *snapshot = get_sources(client, &error);
  if (!snapshot) {
    return error;
  }
  const pulse_source_ref_t *next = pulse_select_next_source(
      snapshot->sources, snapshot->count, snapshot->default_source_name);
  if (!next) {
    snapshot_free(snapshot);
    return error_with_code(PULSE_ERROR_NO_SOURCES, NULL);
  }

  error = set_default_source(client, next->name);
  snapshot_free(snapshot);
  return error;
}

/**
 * public
 */
pulse_client_t *pulse_client_new(void) {
  pulse_client_t *client = calloc(1, sizeof(pulse_client_t));
  if (!client) {
    return NULL;
  }

  client->mainloop = pa_threaded_mainloop_new();
  if (!client->mainloop) {
    free(client);
    return NULL;
  }

  client->context =
      pa_context_new(pa_threaded_mainloop_get_api(client->mainloop),
                     "waybar-pulseaudio-sources");
  if (!client->context) {
    pa_threaded_mainloop_free(client->mainloop);
    free(client);
    return NULL;
  }

  if (pa_threaded_mainloop_start(client->mainloop) == 0) {
    return client;
  }

  pa_context_unref(client->context);
  pa_threaded_mainloop_free(client->mainloop);
  free(client);
  return NULL;
}

void pulse_client_free(pulse_client_t *client) {
  pa_threaded_mainloop_lock(client->mainloop);
  pa_context_disconnect(client->context);
  pa_threaded_mainloop_unlock(client->mainloop);

  pa_threaded_mainloop_stop(client->mainloop);
  pa_context_unref(client->context);
  pa_threaded_mainloop_free(client->mainloop);
  free(client);
}

pulse_error_t pulse_client_start(pulse_client_t *client) {
  return client_with_lock(client, client_start);
}

void pulse_source_free(pulse_source_t *source) {
  source_free_fields(source);
  free(source);
}

pulse_source_t *pulse_get_default_source(pulse_client_t *client,
                                         pulse_error_t *error) {
  pulse_source_t *source = calloc(1, sizeof(pulse_source_t));
  if (!source) {
    *error = error_with_code(PULSE_ERROR_DEFAULT_SOURCE, NULL);
    return NULL;
  }

  *error = client_lock_ready(client, get_default_source, source);
  if (error_failed(*error)) {
    pulse_source_free(source);
    return NULL;
  }

  return source;
}

pulse_error_t pulse_cycle_default_source(pulse_client_t *client) {
  return client_lock_ready(client, set_next_source_default, NULL);
}

pulse_error_t pulse_wait_for_change(pulse_client_t *client) {
  return client_lock_ready(client, wait_for_change, NULL);
}

void pulse_client_shutdown(pulse_client_t *client) {
  pa_threaded_mainloop_lock(client->mainloop);
  client->shutdown = true;
  pa_threaded_mainloop_signal(client->mainloop, 0);
  pa_threaded_mainloop_unlock(client->mainloop);
}
