// Declares a dashboard panel for a custom event you're already tracking:
// the dashboard counts `calculator_used` events per distinct value of the
// `from_currency` metadata key over the selected range. Copy into
// PLUGINS_DIR, rename eventName/groupBy to match your own event, and
// restart — see docs/plugins.md for the full panel spec and its current
// limitations.

function registerPanel() {
  return {
    title: "Calculator usage by currency",
    eventName: "calculator_used",
    chart: "table",
    groupBy: "from_currency"
  };
}
