// Declares a dashboard panel for a custom event you're already tracking.
// Copy into PLUGINS_DIR, rename eventName to match your own event, and
// restart — see docs/plugins.md for the full panel spec and its current
// limitations (the dashboard doesn't compute groupBy aggregates yet).

function registerPanel() {
  return {
    title: "Calculator usage",
    eventName: "calculator_used",
    chart: "line",
    groupBy: "amount"
  };
}
