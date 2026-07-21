// sudoku-flow SPA. F-11: ALL response data (solution digits, technique tags, witness cells,
// status, the metric quartet) and the echoed input are written to the DOM via textContent /
// createTextNode / element-property assignment ONLY. There is no innerHTML, insertAdjacentHTML,
// or document.write anywhere in this file. Grep this file for "innerHTML" — you will find zero.
"use strict";

const CELLS = 81;
const EMPTY = "0";

// --- grid construction -------------------------------------------------------------------

const gridEl = document.getElementById("grid");
const inputs = [];

for (let i = 0; i < CELLS; i++) {
  const row = Math.floor(i / 9);
  const col = i % 9;
  const cell = document.createElement("input");
  cell.className = "cell";
  cell.type = "text";
  cell.inputMode = "numeric";
  cell.maxLength = 1;
  cell.setAttribute("aria-label", `Row ${row + 1} column ${col + 1}`);
  if (row % 3 === 0) cell.classList.add("seam-top");
  if (col % 3 === 0) cell.classList.add("seam-left");
  cell.addEventListener("input", onCellInput);
  cell.addEventListener("paste", onPaste);
  inputs.push(cell);
  gridEl.appendChild(cell);
}

// Only 1-9 stays; anything else clears the cell. Typing also drops any prior solved styling.
function onCellInput(e) {
  const v = e.target.value.replace(/[^1-9]/g, "");
  e.target.value = v;
  e.target.classList.remove("solved");
  e.target.removeAttribute("readonly");
}

// Pasting an 81-char (or longer) string anywhere distributes it across the whole grid. '0' and
// '.' are read as empty. Shorter pastes fall through to the single-cell default.
function onPaste(e) {
  const text = (e.clipboardData || window.clipboardData).getData("text") || "";
  const cleaned = text.replace(/\s+/g, "");
  if (cleaned.length >= CELLS) {
    e.preventDefault();
    loadPuzzle(cleaned.slice(0, CELLS));
  }
}

function loadPuzzle(s) {
  for (let i = 0; i < CELLS; i++) {
    const ch = s[i];
    const cell = inputs[i];
    cell.classList.remove("solved");
    cell.removeAttribute("readonly");
    cell.value = ch >= "1" && ch <= "9" ? ch : "";
  }
}

function readGrid() {
  let out = "";
  for (let i = 0; i < CELLS; i++) {
    const v = inputs[i].value;
    out += v >= "1" && v <= "9" ? v : EMPTY;
  }
  return out;
}

// --- controls ----------------------------------------------------------------------------

const solveBtn = document.getElementById("solve");
const clearBtn = document.getElementById("clear");
const statusEl = document.getElementById("status");
const resultEl = document.getElementById("result");
const metricsEl = document.getElementById("metrics");
const logEl = document.getElementById("log");
const eventCountEl = document.getElementById("event-count");

clearBtn.addEventListener("click", () => {
  loadPuzzle(EMPTY.repeat(CELLS));
  resultEl.hidden = true;
  setStatus("", false);
});

solveBtn.addEventListener("click", solve);

function setStatus(msg, isErr) {
  statusEl.textContent = msg; // textContent: never innerHTML.
  statusEl.classList.toggle("err", !!isErr);
}

async function solve() {
  const puzzle = readGrid();
  solveBtn.disabled = true;
  setStatus("Solving…", false);
  resultEl.hidden = true;
  try {
    const resp = await fetch("/v1/solve", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ puzzle }),
    });
    const data = await resp.json();
    if (!resp.ok) {
      setStatus(data && data.error ? data.error : `Request failed (${resp.status})`, true);
      return;
    }
    render(data);
  } catch (err) {
    setStatus("Network error: " + String(err && err.message ? err.message : err), true);
  } finally {
    solveBtn.disabled = false;
  }
}

// --- rendering (F-11: textContent / createTextNode / property assignment only) ------------

function render(data) {
  setStatus(data.solved ? "Solved" : `Status: ${data.status}`, !data.solved);
  paintSolution(readGrid(), typeof data.solution === "string" ? data.solution : "");
  paintMetrics(data);
  paintLog(Array.isArray(data.events) ? data.events : []);
  resultEl.hidden = false;
}

// Fill the grid from the solution string. Cells the solver placed (empty in the input, filled
// in the solution) render in the accent and go readonly; givens keep the ink color.
function paintSolution(input, solution) {
  for (let i = 0; i < CELLS; i++) {
    const cell = inputs[i];
    const wasGiven = input[i] >= "1" && input[i] <= "9";
    const sol = solution[i];
    if (sol >= "1" && sol <= "9") {
      cell.value = sol; // property assignment, not markup.
      cell.setAttribute("readonly", "readonly");
      cell.classList.toggle("solved", !wasGiven);
    }
  }
}

function paintMetrics(data) {
  metricsEl.replaceChildren();
  const quartet = [
    ["iterations", data.iterations],
    ["events", data.eventCount],
    ["candidate checks", data.candidateChecks],
    ["solve time ms", data.solveTimeMs],
  ];
  for (const [label, value] of quartet) {
    metricsEl.appendChild(metric(label, value));
  }
}

function metric(label, value) {
  const wrap = document.createElement("div");
  wrap.className = "metric";
  const k = document.createElement("span");
  k.className = "k";
  k.textContent = label; // textContent.
  const v = document.createElement("span");
  v.className = "v";
  v.textContent = String(value ?? ""); // textContent.
  wrap.append(k, v);
  return wrap;
}

function paintLog(events) {
  logEl.replaceChildren();
  eventCountEl.textContent = String(events.length); // textContent.
  for (const ev of events) {
    logEl.appendChild(logRow(ev));
  }
}

function logRow(ev) {
  const li = document.createElement("li");

  const tech = document.createElement("span");
  tech.className = "tech";
  tech.textContent = String(ev.technique ?? "?"); // technique tag via textContent.

  const eff = effectText(ev);
  const witness = witnessText(ev.witnessCells);

  li.append(document.createTextNode(`#${numOr(ev.seq)} `), tech);
  if (eff) li.appendChild(document.createTextNode(" — " + eff));
  if (witness) {
    const w = document.createElement("span");
    w.className = "witness";
    w.textContent = "  witness: " + witness; // witness cells via textContent.
    li.appendChild(w);
  }
  return li;
}

function effectText(ev) {
  if (ev.placement && ev.placement.cell) {
    const c = ev.placement.cell;
    return `place ${numOr(ev.placement.value)} at ${cellName(c)}`;
  }
  if (Array.isArray(ev.eliminations) && ev.eliminations.length > 0) {
    const parts = ev.eliminations.map((e) => `${numOr(e.candidate)}@${cellName(e.cell)}`);
    return "eliminate " + parts.join(", ");
  }
  return "";
}

function witnessText(cells) {
  if (!Array.isArray(cells) || cells.length === 0) return "";
  return cells.map(cellName).join(", ");
}

function cellName(c) {
  if (!c) return "?";
  return `r${numOr(c.row)}c${numOr(c.col)}`;
}

function numOr(n) {
  return typeof n === "number" ? String(n) : "?";
}
