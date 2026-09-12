/* FinancesGo — UI behaviors: charts, dialogs, form prefill. */
(function () {
  "use strict";

  const PURPLE = "#8B5CF6";
  const PURPLE2 = "#A855F7";
  const GREEN = "#34D399";
  const RED = "#F87171";
  const MUTED = "#9A93A8";
  const GRID = "rgba(255,255,255,0.06)";

  function brl(v) {
    return "R$ " + Number(v).toLocaleString("pt-BR", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  }

  function baseOpts(extra) {
    return Object.assign({
      responsive: true,
      maintainAspectRatio: false,
      plugins: {
        legend: { labels: { color: MUTED, usePointStyle: true, boxWidth: 8, font: { size: 12 } } },
        tooltip: {
          backgroundColor: "#221D2B", borderColor: "#2A2533", borderWidth: 1,
          titleColor: "#fff", bodyColor: "#ECEAF0", padding: 10, cornerRadius: 8,
          callbacks: { label: (c) => " " + (c.dataset.label ? c.dataset.label + ": " : "") + brl(c.parsed.y ?? c.parsed) }
        }
      }
    }, extra || {});
  }

  function axisOpts() {
    return {
      x: { ticks: { color: MUTED, font: { size: 11 } }, grid: { color: GRID } },
      y: { ticks: { color: MUTED, font: { size: 11 }, callback: (v) => "R$ " + v }, grid: { color: GRID } }
    };
  }

  function parse(el, attr) {
    try { return JSON.parse(el.getAttribute(attr) || "[]"); } catch (e) { return []; }
  }

  function initCharts() {
    if (typeof Chart === "undefined") return;
    Chart.defaults.font.family = "Inter, system-ui, sans-serif";

    const flow = document.getElementById("chart-cashflow");
    if (flow) {
      new Chart(flow, {
        type: "bar",
        data: {
          labels: parse(flow, "data-labels"),
          datasets: [
            { label: "Entradas", data: parse(flow, "data-inflow"), backgroundColor: GREEN, borderRadius: 6, maxBarThickness: 26 },
            { label: "Saídas", data: parse(flow, "data-outflow"), backgroundColor: RED, borderRadius: 6, maxBarThickness: 26 }
          ]
        },
        options: baseOpts({ scales: axisOpts() })
      });
    }

    const cat = document.getElementById("chart-categories");
    if (cat) {
      const labels = parse(cat, "data-labels");
      const values = parse(cat, "data-values");
      if (labels.length) {
        new Chart(cat, {
          type: "doughnut",
          data: {
            labels: labels,
            datasets: [{
              data: values,
              backgroundColor: [PURPLE, PURPLE2, "#EC4899", "#60A5FA", "#34D399", "#FBBF24", "#F87171", "#22D3EE", "#A3E635"],
              borderColor: "#151318", borderWidth: 2
            }]
          },
          options: baseOpts({ cutout: "62%" })
        });
      }
    }
  }

  /* Dialogs --------------------------------------------------------------- */
  window.openDialog = function (id) {
    const d = document.getElementById(id);
    if (d && typeof d.showModal === "function") d.showModal();
  };
  window.closeDialog = function (id) {
    const d = document.getElementById(id);
    if (d) d.close();
  };

  /* Pokémon action dialogs ------------------------------------------------ */
  function accountsMap() {
    const el = document.getElementById("accounts-data");
    if (!el) return {};
    try {
      const list = JSON.parse(el.getAttribute("data-json") || "[]") || [];
      const map = {};
      list.forEach((a) => { map[a.ID] = a; });
      return map;
    } catch (e) { return {}; }
  }

  function setVal(form, name, value) {
    const f = form.querySelector('[name="' + name + '"]');
    if (f != null) f.value = value == null ? "" : value;
  }

  window.editAccount = function (id) {
    const acc = accountsMap()[id];
    if (!acc) return;
    const dlg = document.getElementById("dlg-edit");
    const form = dlg.querySelector("form");
    form.action = "/pokemon/" + id + "/update";
    setVal(form, "email", acc.Email);
    setVal(form, "level", acc.Level);
    setVal(form, "team", acc.Team);
    setVal(form, "description", acc.Description);
    setVal(form, "legendaries", acc.Legendaries);
    setVal(form, "shinies", acc.Shinies);
    setVal(form, "pokemon_count", acc.PokemonCount);
    setVal(form, "bag_capacity", acc.BagCapacity);
    setVal(form, "base_value", acc.BaseValue);
    setVal(form, "ggmax_rate", (acc.GGMaxRate * 100).toFixed(2));
    setVal(form, "tags", acc.Tags);
    updateEditPreview();
    dlg.showModal();
  };

  window.sellAccount = function (id) {
    const acc = accountsMap()[id];
    const dlg = document.getElementById("dlg-sell");
    const form = dlg.querySelector("form");
    form.action = "/pokemon/" + id + "/sell";
    if (acc) {
      const final = acc.BaseValue * (1 + acc.GGMaxRate);
      setVal(form, "value", final.toFixed(2));
      const info = dlg.querySelector(".sell-info");
      if (info) info.textContent = acc.Email || ("Conta #" + id);
    }
    dlg.showModal();
  };

  window.problemAccount = function (id, isProblem) {
    const dlg = document.getElementById("dlg-problem");
    const form = dlg.querySelector("form");
    form.action = "/pokemon/" + id + "/problem";
    const title = dlg.querySelector(".dlg-problem-title");
    if (title) title.textContent = isProblem ? "Remover problema da conta" : "Marcar problema na conta";
    dlg.showModal();
  };

  window.refundAccount = function (id) {
    const dlg = document.getElementById("dlg-refund");
    const form = dlg.querySelector("form");
    form.action = "/pokemon/" + id + "/refund";
    dlg.showModal();
  };

  /* Live GGMAX preview on create/edit forms ------------------------------- */
  function computePreview(form) {
    if (!form) return;
    const baseEl = form.querySelector('[name="base_value"]');
    const rateEl = form.querySelector('[name="ggmax_rate"]');
    const out = form.querySelector(".price-preview");
    if (!baseEl || !out) return;
    const base = parseFloat((baseEl.value || "0").replace(",", ".")) || 0;
    let rate = parseFloat((rateEl && rateEl.value || "0").replace(",", ".")) || 0;
    if (rate > 1) rate = rate / 100;
    out.textContent = brl(base * (1 + rate));
  }
  window.updateCreatePreview = function () { computePreview(document.getElementById("form-create")); };
  window.updateEditPreview = function () { computePreview(document.querySelector("#dlg-edit form")); };

  document.addEventListener("input", function (e) {
    if (e.target.name === "base_value" || e.target.name === "ggmax_rate") {
      computePreview(e.target.closest("form"));
    }
  });

  /* Auto-submit filter selects ------------------------------------------- */
  document.addEventListener("change", function (e) {
    if (e.target.matches("[data-autosubmit]")) {
      e.target.closest("form").submit();
    }
  });

  document.addEventListener("DOMContentLoaded", initCharts);
})();
