(function () {
  function renderTimeseriesChart() {
    var el = document.getElementById("timeseries-chart");
    if (!el || !window.uPlot) return;

    var raw = [];
    try {
      raw = JSON.parse(el.dataset.points || "[]");
    } catch (e) {
      return;
    }

    var xs = raw.map(function (p) { return Math.floor(new Date(p.bucket).getTime() / 1000); });
    var ys = raw.map(function (p) { return p.count; });

    el.innerHTML = "";
    new uPlot(
      {
        width: el.clientWidth || 600,
        height: 240,
        series: [{}, { label: "Eventos", stroke: "#6ee7b7", width: 2 }],
      },
      [xs, ys],
      el
    );
  }

  document.addEventListener("DOMContentLoaded", renderTimeseriesChart);
  // Re-render after any htmx swap (filter changes reload #overview), since
  // the chart's canvas gets torn down along with the swapped-out markup.
  document.body.addEventListener("htmx:afterSwap", renderTimeseriesChart);
})();
