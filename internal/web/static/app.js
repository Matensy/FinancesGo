/* FinancesGo — UI behaviors: charts, dialogs, form prefill. */
(function () {
  "use strict";

  const PURPLE = "#7c5cff";
  const PURPLE2 = "#a855f7";
  const GREEN = "#2dd4a7";
  const RED = "#ff5d7a";
  const MUTED = "#7f8397";
  const GRID = "rgba(255,255,255,0.05)";

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
              backgroundColor: [PURPLE, PURPLE2, "#ec4899", "#4d8dff", "#2dd4a7", "#f5b942", "#ff5d7a", "#22d3ee", "#a3e635"],
              borderColor: "#14161f", borderWidth: 3
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

  /* GGMAX-style description formatter ------------------------------------- */
  const TEAMS = {
    valor: { emoji: "🟥", name: "Team Red" },
    mystic: { emoji: "🟦", name: "Team Blue" },
    instinct: { emoji: "🟨", name: "Team Yellow" }
  };

  function titleWord(w) {
    if (/[0-9%]/.test(w)) return w; // keep "100%", "1,1KK"
    return w.split("-").map((p) => (p ? p[0].toUpperCase() + p.slice(1).toLowerCase() : p)).join("-");
  }

  function formatName(seg) {
    seg = seg.trim();
    if (!seg) return "";
    if (/^etc\.?$/i.test(seg)) return "etc.";
    let paren = "";
    const m = seg.match(/\(([^)]*)\)/);
    if (m) {
      paren = " (" + m[1].trim().toUpperCase() + ")";
      seg = seg.replace(/\([^)]*\)/, "").trim();
    }
    const main = seg.split(/\s+/).filter(Boolean).map(titleWord).join(" ");
    return (main + paren).trim();
  }

  const MARK_EMOJIS = ["🟥", "🟦", "🟨", "📦", "⭐", "✨", "🐉"];

  // extractNames pulls just the pokémon names out of the description, ignoring
  // any header/count lines a previous format run may have added (idempotent).
  function extractNames(raw) {
    const kept = raw.split(/\r?\n/).filter((line) => {
      const t = line.trim();
      if (!t) return false;
      if (MARK_EMOJIS.some((e) => t.startsWith(e))) return false;
      if (/pok[eé]mons lend[aá]rios e shinys/i.test(t)) return false;
      if (/^level\b/i.test(t)) return false;
      if (/^\d+\s*\/\s*\d+\s*pok/i.test(t)) return false;
      if (/lend[aá]rios?\s*\/\s*\d+\s*shin/i.test(t)) return false;
      if (/stardust/i.test(t)) return false;
      return true;
    });
    return kept.join("/");
  }

  function normalizeStardust(s) {
    s = (s || "").trim().replace(/\.$/, "");
    if (!s) return "";
    s = s.replace(/kk/gi, "KK").replace(/\bk\b/gi, "K");
    if (!/stardust/i.test(s)) s += " de Stardust";
    else s = s.replace(/\bstardust\b/i, "Stardust");
    return s;
  }

  // formatDescription builds the GGMAX-style listing using the already-filled
  // form fields for level/team/counts/stardust; the description holds only the
  // pokémon names.
  function formatDescription(raw, f) {
    const names = extractNames(raw).split(/[\/,]/).map(formatName).filter(Boolean);
    const t = TEAMS[f.team] || null;
    const out = [];

    let header = "";
    if (t) header += t.emoji + " ";
    header += "Level " + (f.level || "?");
    if (t) header += " (" + t.name + ")";
    out.push(header);

    if (f.pokemonCount || f.bagCapacity) {
      out.push("📦 " + (f.pokemonCount || "?") + "/" + (f.bagCapacity || "?") + " Pokémons");
    }
    const lendNum = String(f.legendaries || "").trim();
    const shiny = String(f.shinies || "").trim();
    let counts = "";
    if (/^\d+$/.test(lendNum) && shiny) counts = lendNum + " Lendários / " + shiny + " Shinys";
    else if (shiny) counts = shiny + " Shinys";
    else if (/^\d+$/.test(lendNum)) counts = lendNum + " Lendários";
    if (counts) out.push("⭐ " + counts);

    const star = normalizeStardust(f.stardust);
    if (star) out.push("✨ " + star);

    if (names.length) {
      out.push("");
      out.push("🐉 Pokémons Lendários e Shinys");
      out.push(names.join(", "));
    }
    return out.join("\n");
  }

  window.formatDesc = function (btn) {
    const form = btn.closest("form");
    if (!form) return;
    const desc = form.querySelector('[name="description"]');
    if (!desc) return;
    const val = (n) => { const el = form.querySelector('[name="' + n + '"]'); return el ? el.value : ""; };
    desc.value = formatDescription(desc.value, {
      level: val("level"),
      team: val("team"),
      shinies: val("shinies"),
      pokemonCount: val("pokemon_count"),
      bagCapacity: val("bag_capacity"),
      legendaries: val("legendaries"),
      stardust: val("stardust")
    });
  };

  document.addEventListener("DOMContentLoaded", initCharts);
})();
