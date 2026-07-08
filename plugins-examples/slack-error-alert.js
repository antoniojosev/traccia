// Posts to a Slack (or Discord, or any JSON-webhook) URL whenever an
// "error" event comes in. Copy into PLUGINS_DIR and set the URL below.
//
// See docs/plugins.md for the full sandbox API (log/http/kv) and the
// limitations that apply (~100ms time budget, no filesystem/network
// beyond http.post).

var WEBHOOK_URL = "https://hooks.slack.com/services/CHANGE/ME/NOW";

function onEvent(event) {
  if (event.type === "error") {
    http.post(WEBHOOK_URL, {
      text: "Traccia error: " + event.name + " on " + event.path
    });
    log.info("notified Slack about " + event.name);
  }
  return event;
}
